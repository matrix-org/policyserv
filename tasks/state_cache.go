package tasks

import (
	"context"
	"log"
	"time"

	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/storage"
)

func CacheLearnedRoomState(homeserver *homeserver.Homeserver, db storage.PersistentStorage) {
	log.Println("Running learn state task...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	val, txn, err := db.PopStateLearnQueue(ctx)
	if txn != nil {
		defer txn.Rollback() // if something goes wrong, just roll back.
	}
	if err != nil {
		log.Printf("Non-fatal error popping state learn queue: %v", err)
		return
	}
	if val == nil {
		log.Println("No state to learn")
		return // no work to do
	}

	log.Printf("Learning state: %#v", val)
	err = homeserver.LearnStateIfExpired(ctx, val.RoomId, val.AtEventId, val.ViaServer)
	if err != nil {
		log.Printf("Non-fatal error learning state in %s at %s via %s: %v", val.RoomId, val.AtEventId, val.ViaServer, err)
		return
	}
	log.Printf("State learned: %#v", val)

	err = txn.Commit()
	if err != nil {
		log.Printf("Non-fatal error committing transaction: %v", err)
		return
	}

	log.Println("Finished learn state task")
}
