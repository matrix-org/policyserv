package homeserver

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
)

// GetEvent - Fetches an event over federation from the listed servers. The first server to return a succesful
// result is used. If *all* servers return an error, a joined error is returned. If one server returns an error,
// but another server is successful, the error is discarded.
func (h *Homeserver) GetEvent(ctx context.Context, eventId string, via []string) (gomatrixserverlib.PDU, error) {
	// We're about to hit all servers concurrently, so set up a dedicated cancelable context for those hits.
	// We also defer the cancellation right away so we can stop other HTTP calls once we get a successful result.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(via))
	pduCh := make(chan gomatrixserverlib.PDU, 1)
	wg := &sync.WaitGroup{}
	wg.Add(len(via))

	defer func() {
		// close channels after the wait group is done to avoid errors, but also don't wait forever
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		select {
		case <-ctx.Done():
		case <-internal.WaitGroupDone(wg):
		}

		close(errCh)
		close(pduCh)
	}()

	for _, serverName := range via {
		go func(serverName string) {
			defer wg.Done()

			// When the wait group finishes, we might be aggregating errors on errCh. We need to ensure we send
			// something on that channel to avoid deadlocking that aggregation. We also need to be sure we send
			// *exactly* one value, so we don't take a "slot" from another gofunc trying to do the same thing.
			//
			// ⚠️ It's important that code which calls sendErr also returns so sendErr can't be called twice.
			didSendErr := false
			defer func() {
				if !didSendErr {
					errCh <- nil
				}
			}()
			sendErr := func(err error) {
				didSendErr = true
				errCh <- err
			}

			txn, err := h.client.GetEvent(ctx, h.ServerName, spec.ServerName(serverName), eventId)
			if err != nil {
				sendErr(err)
				return
			}
			if len(txn.PDUs) == 1 {
				raw := txn.PDUs[0]

				// Figure out what room version the event is
				header := eventRoomIdOnly{}
				err = json.Unmarshal(raw, &header)
				if err != nil {
					sendErr(err)
					return
				}
				var room *storage.StoredRoom
				room, err = h.storage.GetRoom(ctx, header.RoomId)
				if err != nil {
					sendErr(err)
					return
				}
				if room == nil {
					sendErr(errors.New("room not found"))
					return
				}
				roomVersion := gomatrixserverlib.MustGetRoomVersion(gomatrixserverlib.RoomVersion(room.RoomVersion))

				// Now we can parse the event, hopefully
				var event gomatrixserverlib.PDU
				event, err = roomVersion.NewEventFromUntrustedJSON(raw)
				if err != nil {
					sendErr(err)
					return
				}

				// Verify the event signatures before considering the request "successful"
				if err = gomatrixserverlib.VerifyEventSignatures(ctx, event, h.keyRing, func(roomId spec.RoomID, senderId spec.SenderID) (*spec.UserID, error) {
					return senderId.ToUserID(), nil
				}); err != nil {
					sendErr(err)
					return
				}

				// *now* we can consider the request successful
				pduCh <- event
			} else {
				sendErr(errors.New("event not found (or too many events returned)"))
				return
			}
		}(serverName)
	}

	// Wait for either pduCh to get an event or for the wait group to finish completely
	select {
	case pdu := <-pduCh:
		return pdu, nil
	case <-internal.WaitGroupDone(wg):
		err := errors.New("unable to fetch event")
		for i := 0; i < len(via); i++ {
			err = errors.Join(err, <-errCh)
		}
		return nil, err
	}
}
