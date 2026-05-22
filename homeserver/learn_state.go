package homeserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
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
