package main

import (
	"crypto/rand"
	"log"
	"math/big"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/tasks"
	"github.com/go-co-op/gocron/v2"
)

func setupScheduler(scheduler gocron.Scheduler, db storage.PersistentStorage, instanceConfig *config.InstanceConfig) error {
	if err := scheduleMuninnTask(scheduler, db, instanceConfig); err != nil {
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

	// We schedule the first run to be pretty quick, but with some jitter to avoid overlapping calls here too
	go func() {
		// we don't *need* a cryptographic random number here, but security audits might complain if we don't
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			log.Printf("Non-fatal error generating jitter for Muninn Hall cache task: %v", err)
			n = big.NewInt(4) // https://xkcd.com/221
		}
		<-time.After(time.Duration(n.Int64()) * time.Second)
		if err = muninnTask.RunNow(); err != nil {
			log.Printf("Non-fatal error trying to run Muninn Hall cache task immediately: %v", err)
		}
	}()
	log.Printf("Scheduled Muninn Hall cache task every hour: %s", muninnTask.ID())

	return nil
}
