package main

import (
	"crypto/rand"
	"log"
	"math"
	"math/big"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/tasks"
)

func setupScheduler(scheduler gocron.Scheduler, homeserver *homeserver.Homeserver, db storage.PersistentStorage, instanceConfig *config.InstanceConfig) error {
	if err := scheduleMuninnTask(scheduler, db, instanceConfig); err != nil {
		return err
	}
	if err := scheduleStateLearningTask(scheduler, homeserver, db, instanceConfig); err != nil {
		return err
	}
	if err := scheduleFederationCatchupTask(scheduler, homeserver, db, instanceConfig); err != nil {
		return err
	}
	return nil
}

func scheduleMuninnTask(scheduler gocron.Scheduler, db storage.PersistentStorage, instanceConfig *config.InstanceConfig) error {
	// We schedule this to run every hour +/- 10 minutes to avoid overlapping calls from other processes that might get us rate limited.
	muninnTask, err := scheduler.NewJob(gocron.DurationRandomJob(50*time.Minute, 70*time.Minute), gocron.NewTask(tasks.CacheMuninnHallMembers, db, instanceConfig), gocron.WithName("CacheMuninnHallMembers"))
	if err != nil {
		return err
	}

	log.Printf("Scheduled Muninn Hall cache task every hour: %s", muninnTask.ID())
	runTaskNowish(muninnTask)

	return nil
}

func scheduleStateLearningTask(scheduler gocron.Scheduler, homeserver *homeserver.Homeserver, db storage.PersistentStorage, instanceConfig *config.InstanceConfig) error {
	if instanceConfig.StateCacheIntervalMinutes <= 0 {
		log.Printf("PS_STATE_CACHE_INTERVAL_MINUTES must be greater than 0. Using default of 60 minutes.")
		instanceConfig.StateCacheIntervalMinutes = 60
	}

	// We do the math in seconds to get a slightly more accurate number (10% of 1 minute is 6 seconds, but if we did our
	// math in minutes then we'd end up with a range of 1 minute).
	variance := time.Duration(float64(instanceConfig.StateCacheIntervalMinutes*60)*0.1) * time.Second
	minMinutes := (time.Duration(instanceConfig.StateCacheIntervalMinutes) * time.Minute) - variance
	maxMinutes := (time.Duration(instanceConfig.StateCacheIntervalMinutes) * time.Minute) + variance

	// "should never happen" clauses
	if minMinutes < 0 {
		minMinutes = 1 * time.Minute
	}
	if maxMinutes < minMinutes {
		maxMinutes = minMinutes + time.Minute
	}

	learnTask, err := scheduler.NewJob(gocron.DurationRandomJob(minMinutes, maxMinutes), gocron.NewTask(tasks.CacheLearnedRoomState, homeserver, db), gocron.WithName("LearnRoomState"))
	if err != nil {
		return err
	}

	log.Printf("Scheduled state learning task every ~%d minutes: %s", instanceConfig.StateCacheIntervalMinutes, learnTask.ID())
	runTaskNowish(learnTask)

	return nil
}

func scheduleFederationCatchupTask(scheduler gocron.Scheduler, homeserver *homeserver.Homeserver, db storage.PersistentStorage, instanceConfig *config.InstanceConfig) error {
	if instanceConfig.FederationCatchupIntervalSeconds <= 0 {
		// We *really* want people to see this, but it's questionable to even allow this feature to be disabled at all.
		// There may be situations where sending more transactions won't fix a bug (typically on the remote end), so we
		// give operators a way to just stop all traffic.
		for range 5 {
			log.Printf("⚠️ Federation catchup is disabled. Remote servers might not receive policyserv's outbound traffic, and your database will accumulate data. Set PS_FEDERATION_CATCHUP_INTERVAL_SECONDS to a positive number to re-enable.")
		}
		return nil
	}

	// Jitter on these requests is to avoid other policyserv instances from trying to acquire locks at the exact same
	// time. This also means that the mean delivery time of a failed transaction will actually be less than the configured
	// interval because another worker is likely to pick it up first.
	minTime := time.Second * time.Duration(math.Max(1, float64(instanceConfig.FederationCatchupIntervalSeconds)-2))
	maxTime := time.Second * time.Duration(instanceConfig.FederationCatchupIntervalSeconds+2)

	catchupTask, err := scheduler.NewJob(gocron.DurationRandomJob(minTime, maxTime), gocron.NewTask(tasks.FederationCatchup, homeserver, db), gocron.WithName("FederationCatchup"))
	if err != nil {
		return err
	}

	log.Printf("Scheduled federation catchup task every ~%d seconds: %s", instanceConfig.FederationCatchupIntervalSeconds, catchupTask.ID())
	if instanceConfig.FederationCatchupIntervalSeconds > 30 {
		// Somewhat arbitrary, but if the interval isn't "soon" then we run the task right away. Otherwise we just wait it out.
		runTaskNowish(catchupTask)
	}

	return nil
}

// runTaskNowish - Runs a gocron task as quickly as possible, with a small delay to avoid overlapping calls. The task will
// wait asynchronously to run, so this will return immediately regardless of whether the task is running.
func runTaskNowish(task gocron.Job) {
	go func() {
		// we don't *need* a cryptographic random number here, but security audits might complain if we don't
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			log.Printf("Non-fatal error generating jitter for task %s: %v", task.ID(), err)
			n = big.NewInt(4) // https://xkcd.com/221
		}
		<-time.After(time.Duration(n.Int64()) * time.Second)
		if err = task.RunNow(); err != nil {
			log.Printf("Non-fatal error trying to run task %s immediately: %v", task.ID(), err)
		}
	}()
}
