package tasks

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/storage"
	"github.com/panjf2000/ants/v2"
)

func FederationCatchup(homeserver *homeserver.Homeserver, db storage.PersistentStorage) {
	log.Println("Running federation catchup task...")

	// Our database query should be pretty quick, so set a quick timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	destinations, err := db.GetDestinationsNeedingCatchup(ctx)
	if err != nil {
		log.Printf("Failed to get destinations needing catchup: %v", err)
		return
	}

	if len(destinations) == 0 {
		log.Println("No destinations need catchup")
		return
	}

	// We set up a small pool to avoid processing all destinations all at once
	pool, err := ants.NewPoolWithFuncGeneric[string](10, func(destination string) {
		log.Printf("Sending federation catchup to %s", destination)

		// This can take a while, so we don't want to tie it to our short context above. It will
		// deduplicate requests for us, and will also limit its own execution time.
		homeserver.SendNextTransactionTo(context.Background(), destination)
	})
	if err != nil {
		log.Printf("Failed to create pool for federation catchup: %v", err)
		return
	}

	wg := &sync.WaitGroup{}
	for _, destination := range destinations {
		log.Printf("Destination %s is behind on federation and needs catchup", destination)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// we do this async because Invoke() will block for an available worker
			err = pool.Invoke(destination)
			if err != nil {
				log.Printf("Failed to send federation catchup to %s: %v", destination, err)
			}
		}()
	}

	// Wait for a bit for the pool to naturally self-close before force-closing it
	wg.Wait() // we want to wait for all the jobs to be submitted first
	for i := 0; i < 5; i++ {
		err = pool.ReleaseTimeout(1 * time.Minute)
		if err == nil {
			return // we closed successfully
		}
		if errors.Is(err, ants.ErrTimeout) {
			continue // try again
		}
		log.Printf("Unexpected error closing federation catchup pool: %v", err) // "should never happen"
	}
	pool.Release() // force close the pool
}
