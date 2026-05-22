package homeserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/storage"
)

func (h *Homeserver) shouldLearnState(ctx context.Context, roomId string) (bool, *storage.StoredRoom, error) {
	room, err := h.storage.GetRoom(ctx, roomId)
	if err != nil {
		return false, nil, err
	}
	if room == nil {
		return false, nil, fmt.Errorf("room %s not found", roomId)
	}

	if time.Now().Sub(time.UnixMilli(room.LastCachedStateTimestampMillis)) < h.cacheRoomStateFor {
		return false, room, nil // not expired
	}

	return true, room, nil
}

func (h *Homeserver) LearnState(ctx context.Context, roomId string, atEventId string, via string) error {
	// Note: Previous implementation of this function used shouldLearnState() to only learn state if it was expired.
	// In practice, we're reasonably confident that the fact that there's a queue of state to learn means that it's
	// expired enough, so we always learn state regardless of how long it's been since the last update. Removing the
	// expiration check also fixes an issue where the learn state task could be cleared for a room quietly, despite
	// never actually learning state due to the task firing before the expiration time.

	room, err := h.storage.GetRoom(ctx, roomId)
	if err != nil {
		return err
	}
	if room == nil {
		return fmt.Errorf("room %s not found", roomId)
	}

	log.Printf("Fetching state for %s at %s from %s", roomId, atEventId, via)

	res, err := h.client.LookupState(ctx, h.ServerName, spec.ServerName(via), roomId, atEventId, gomatrixserverlib.RoomVersion(room.RoomVersion))
	if err != nil {
		return err
	}
	pdus := res.GetStateEvents().UntrustedEvents(gomatrixserverlib.RoomVersion(room.RoomVersion))

	err = h.stateLearner.LearnFrom(ctx, room, pdus)
	if err != nil {
		return err
	}

	// Bump the cache timestamp, fetching a fresh `room` in case something changed while we've been processing the state
	room, err = h.storage.GetRoom(ctx, roomId)
	if err != nil {
		return errors.Join(fmt.Errorf("error fetching room %s after processing state", roomId), err)
	}
	if room == nil {
		return fmt.Errorf("room %s not found after processing state", roomId)
	}
	room.LastCachedStateTimestampMillis = time.Now().UnixMilli()
	err = h.storage.UpsertRoom(ctx, room)
	if err != nil {
		return errors.Join(fmt.Errorf("error storing room %s after processing state", roomId), err)
	}

	return nil
}

// queueLearnStateIfNeeded will silently decide if room state should be learned based on the result of a filter request
// and the associated event. If state should be learned, it will be queued for processing.
func (h *Homeserver) queueLearnStateIfNeeded(ctx context.Context, basedOnResult *queue.PoolResult, fromEvent gomatrixserverlib.PDU) {
	// Create a new context so we're not tied to our caller's timeout. We aren't doing network calls here, so we can have
	// something relatively short (we want to fail quickly if the database fails, for example).
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Verify that the event we're about to learn state based on is non-spammy in nature
	if basedOnResult.Err != nil {
		return // we're not interested in this result
	}
	if basedOnResult.IsProbablySpam {
		return // too spammy for us
	}

	// Verify the event was sent by a "trusted" origin so we can learn state from them directly
	trusted := false
	for _, origin := range h.trustedOrigins {
		userId := fromEvent.SenderID().ToUserID()
		if userId == nil {
			log.Printf("Non-user sender '%s' in %s at %s", fromEvent.SenderID(), fromEvent.RoomID().String(), fromEvent.EventID())
			break
		}
		if userId.Domain() == spec.ServerName(origin) {
			trusted = true
			break
		}
	}
	if !trusted {
		return
	}

	// One of two conditions need to pass here: either we need to have expired state that should be updated, or the event
	// being processed is something we can learn from directly (meaning we can avoid waiting for another event to arrive
	// and process the queue - we can just do it right away).
	shouldLearn, room, err := h.shouldLearnState(ctx, fromEvent.RoomID().String())
	if err != nil {
		log.Printf("Error checking if room %s should be learned: %s", fromEvent.RoomID().String(), err)
		return
	}
	if !shouldLearn {
		// we might still be able to learn from the event itself though
		canLearn, err := h.stateLearner.CanLearn(ctx, room, fromEvent)
		if err != nil {
			log.Printf("Error checking if event %s can be learned: %s", fromEvent.EventID(), err)
			return
		}
		if !canLearn {
			return
		}
		log.Printf("Event %s in %s can be learned from directly", fromEvent.EventID(), fromEvent.RoomID().String())
	} else {
		log.Printf("Room %s has expired state that should be updated", fromEvent.RoomID().String())
	}

	// Now we're ready to queue the state learning task
	// We queue the state learning so exactly one process will deal with it, instead of *all* of them.
	// This queue also prevents us from learning state if multiple events come in before we finish
	// learning state (ie: 4 events from matrix.org, all activate this bit of code, leading to 4
	// concurrent "learn state" operations).
	log.Printf("Queueing state learning of %s at %s", fromEvent.RoomID().String(), fromEvent.EventID())
	err = h.storage.PushStateLearnQueue(ctx, &storage.StateLearnQueueItem{
		RoomId:               fromEvent.RoomID().String(),
		AtEventId:            fromEvent.EventID(),
		ViaServer:            string(fromEvent.SenderID().ToUserID().Domain()),
		AfterTimestampMillis: time.Now().Add(2 * time.Minute).UnixMilli(), // give the remote server a bit of time to process the event itself
	})
	if err != nil {
		log.Printf("Error queueing state learning of %s at %s: %s", fromEvent.RoomID().String(), fromEvent.EventID(), err)
	}
}
