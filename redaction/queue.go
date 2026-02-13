package redaction

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/storage"
	"github.com/panjf2000/ants/v2"
)

var pool *ants.Pool
var clients = make([]*Client, 0)

func MakePool(instanceConfig *config.InstanceConfig) {
	if pool != nil {
		return
	}

	var err error
	pool, err = ants.NewPool(instanceConfig.ModerationPoolSize, ants.WithOptions(ants.Options{
		ExpiryDuration:   1 * time.Minute,
		PreAlloc:         false,
		MaxBlockingTasks: 0, // no limit on submissions
		Nonblocking:      false,
		// If we don't supply a panic handler then ants will print a stack trace for us
		//PanicHandler: func(err interface{}) {
		//	log.Println("Panic in pool:", err)
		//},
		Logger:       log.Default(),
		DisablePurge: false,
	}))
	if err != nil {
		log.Fatal(err) // "should never happen"
	}

	for hsDomain, accessToken := range instanceConfig.ModeratorAccessTokens {
		client, err := NewClient(fmt.Sprintf("https://%s", hsDomain), accessToken)
		if err != nil {
			log.Fatal(err)
		}
		clients = append(clients, client)
	}
}

func QueueRedaction(storage storage.PersistentStorage, event gomatrixserverlib.PDU) error {
	metrics.RecordModerationRequest(metrics.ModerationActionRedaction)

	if !event.SenderID().IsUserID() || event.SenderID().ToUserID() == nil {
		log.Printf("Non-user sender '%s' in %s at %s", event.SenderID(), event.RoomID().String(), event.EventID())
		return nil
	}

	roomId := event.RoomID().String()
	eventId := event.EventID()
	senderDomain := string(event.SenderID().ToUserID().Domain())

	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Minute)
	defer cancel()

	room, err := storage.GetRoom(ctx, roomId)
	if err != nil {
		return err
	}
	if room.ModeratorUserId == "" {
		log.Println("No moderator for room", roomId)
		metrics.RecordModerationAction(metrics.ModerationActionRedaction, metrics.ModerationStatusNoModerator)
		return nil // nothing to queue
	}

	for _, client := range clients {
		if client.userId == room.ModeratorUserId {
			if client.domain == senderDomain {
				log.Println("Moderator and sender are on the same homeserver for room", roomId, "and event", eventId, " - skipping redaction (assuming blocked locally)")
				metrics.RecordModerationAction(metrics.ModerationActionRedaction, metrics.ModerationStatusOutOfBandModeration)
				return nil
			}
			workFn := func() {
				doWork(roomId, eventId, client)
			}
			return pool.Submit(workFn)
		}
	}

	log.Println("No client for moderator", room.ModeratorUserId, "in room", roomId, " - skipping redaction")
	metrics.RecordModerationAction(metrics.ModerationActionRedaction, metrics.ModerationStatusModeratorNotConfigured)
	return nil // no clients
}
