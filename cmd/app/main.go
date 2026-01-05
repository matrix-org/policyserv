package main

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/audit"
	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/logging" // import this for side effects if this isn't needed directly anymore
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/redaction"
	"github.com/matrix-org/policyserv/storage"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var err error
	var db storage.PersistentStorage
	var pubsubClient pubsub.Client

	instanceConfig, err := config.NewInstanceConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Start pprof early if configured so startup can be debugged (if needed)
	if instanceConfig.PprofBind != "" {
		go func() {
			// pprof binds itself to the default HTTP server, so we just have to start that server.
			log.Println("Starting pprof server on", instanceConfig.PprofBind)
			log.Fatal(http.ListenAndServe(instanceConfig.PprofBind, nil))
		}()
	}

	// TODO: Remove redaction support (see ModeratorUserID)
	redaction.MakePool(instanceConfig)

	if db, pubsubClient, err = setupDataHandlers(instanceConfig); err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	defer pubsubClient.Close()

	auditQueue, err := audit.NewQueue(instanceConfig.WebhookPoolSize)
	if err != nil {
		log.Fatal(err)
	}

	communityManager, err := setupCommunityManager(instanceConfig, db, pubsubClient, auditQueue)
	if err != nil {
		log.Fatal(err)
	}

	pool, err := setupQueue(instanceConfig, db, communityManager)
	if err != nil {
		log.Fatal(err)
	}

	hs, err := setupHomeserver(instanceConfig, db, pool, pubsubClient)
	if err != nil {
		log.Fatal(err) // "should never happen"
	}
	log.Println("Homeserver name: ", hs.ServerName)
	log.Println("Homeserver KeyID:", hs.KeyId)
	b64 := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hs.GetPublicEventSigningKey())
	log.Println("Public event key:", homeserver.PolicyServerKeyID, b64)

	api, err := setupApi(instanceConfig, db, hs)
	if err != nil {
		log.Fatal(err) // "should never happen"
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	appMux := http.NewServeMux()
	if err = hs.BindTo(appMux); err != nil {
		log.Fatal(err)
	}
	if err = api.BindTo(appMux); err != nil {
		log.Fatal(err)
	}

	metricsServer := &http.Server{Addr: instanceConfig.MetricsBind, Handler: metricsMux}
	appServer := &http.Server{Addr: instanceConfig.HttpBind, Handler: appMux}

	var wg sync.WaitGroup
	stopping := false
	startServer := func(server *http.Server) {
		err := server.ListenAndServe()
		if err != nil && (!stopping && !errors.Is(err, http.ErrServerClosed)) {
			log.Fatal(err)
		}
	}
	stopServer := func(server *http.Server, ctx context.Context) {
		defer wg.Done()
		err := server.Shutdown(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
	wg.Add(2) // 1 for each server
	go startServer(metricsServer)
	go startServer(appServer)

	// Join rooms *after* starting the app server, otherwise the joins will fail due to
	// our key being inaccessible.
	go joinRooms(instanceConfig, hs)

	// Schedule tasks now that we're mostly started up
	scheduler, err := gocron.NewScheduler(gocron.WithLogger(&logging.CronLogger{})) // TODO: Support metrics too (gocron "Monitors")
	if err != nil {
		log.Fatal(err)
	}
	scheduler.Start() // start immediately so we can force jobs to run immediately too
	err = setupScheduler(scheduler, hs, db, instanceConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Wait for a stop signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer close(stop)
	<-stop
	stopping = true

	log.Println("Stopping...")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	go func() {
		cancel()
	}()
	if err = scheduler.StopJobs(); err != nil {
		log.Printf("Failed to stop scheduler: %v", err)
	}
	go stopServer(metricsServer, ctx)
	go stopServer(appServer, ctx)
	wg.Wait()
}

func joinRooms(instanceConfig *config.InstanceConfig, hs *homeserver.Homeserver) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	time.Sleep(15 * time.Second) // wait to give our ingress time to detect us as up
	err := hs.JoinRooms(ctx, instanceConfig.JoinRoomIDs, instanceConfig.JoinServer, "default")
	if err != nil {
		log.Fatal(err)
	}

	// Send a poke to our join server and query server just in case it thinks we're sad
	err = hs.Ping(ctx, instanceConfig.JoinServer)
	if err != nil {
		log.Println("Error pinging join server: ", err)
	}
	err = hs.Ping(ctx, instanceConfig.KeyQueryServer[0])
	if err != nil {
		log.Println("Error pinging key query server: ", err)
	}
}
