package homeserver

import (
	"context"
	"errors"
	"log"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
)

func (h *Homeserver) runFilters(ctx context.Context, event gomatrixserverlib.PDU, waitCh chan<- *queue.PoolResult) error {
	resultCh := make(chan *queue.PoolResult, 1) // a buffered channel reduces the chance of deadlocks

	go func(event gomatrixserverlib.PDU, ch chan *queue.PoolResult, downstream chan<- *queue.PoolResult) {
		defer close(ch)

		// We want to consider the context early on - if the context is cancelled, we want to run through an error path
		var res *queue.PoolResult
		select {
		case res = <-ch:
		case <-ctx.Done():
			err := ctx.Err()
			if err == nil {
				// "should never happen"
				err = errors.New("context was supposed to be cancelled")
			}
			log.Printf("[%s | %s] Request context cancelled, forcing error response: %s", event.EventID(), event.RoomID().String(), err)
			res = &queue.PoolResult{
				Vectors:        nil,
				IsProbablySpam: false,
				Err:            err,
			}
		}

		if downstream != nil {
			// Rebroadcast result async to avoid blocking forever when the downstream channel is unbuffered
			// (we *should* always be supplying a buffered channel, but just in case).
			go func(ctx context.Context, downstream chan<- *queue.PoolResult, result *queue.PoolResult, event gomatrixserverlib.PDU) {
				// First, see if the channel is likely going to be closed already
				if err := ctx.Err(); err != nil {
					log.Printf("[%s | %s] Result channel closed, not rebroadcasting: %s", event.EventID(), event.RoomID().String(), err)
					return
				}

				// Now, try to send the result while considering the context too
				log.Printf("[%s | %s] Rebroadcasting result", event.EventID(), event.RoomID().String())
				select {
				case downstream <- result:
				case <-ctx.Done():
					log.Printf("[%s | %s] Result channel closed, failed rebroadcasting: %s", event.EventID(), event.RoomID().String(), ctx.Err())
				}
			}(ctx, downstream, res, event)
		}

		// We'd like to "learn" room state, if we can. First some checks to make sure we're in a good position to do that.

		if res.Err != nil {
			return // we're not interested in this result
		}
		if res.IsProbablySpam {
			return // too spammy for us
		}
		trusted := false
		for _, origin := range h.trustedOrigins {
			userId := event.SenderID().ToUserID()
			if userId == nil {
				log.Printf("Non-user sender '%s' in %s at %s", event.SenderID(), event.RoomID().String(), event.EventID())
				break
			}
			if userId.Domain() == spec.ServerName(origin) {
				trusted = true
				break
			}
		}
		if !trusted {
			return // not something we're willing to wait for
		}

		// At this point, it's "safe" to learn room state from that event, after we wait a bit for the
		// remote server to finish processing/sending the event. We do that waiting in the background
		// to avoid blocking this request any further (and thus the remote server's ability to finish
		// processing the event).
		go func(h *Homeserver, event gomatrixserverlib.PDU) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			shouldLearn, _, err := h.shouldLearnState(ctx, event.RoomID().String())
			if err != nil {
				log.Printf("Non-fatal error checking if we should learn state for %s: %s", event.RoomID().String(), err)
				return
			}
			if !shouldLearn {
				return
			}

			// We queue the state learning so exactly one process will deal with it, instead of *all* of them.
			// This queue also prevents us from learning state if multiple events come in before we finish
			// learning state (ie: 4 events from matrix.org, all activate this bit of code, leading to 4
			// concurrent "learn state" operations).
			log.Printf("Queueing state learning of %s at %s", event.RoomID().String(), event.EventID())
			err = h.storage.PushStateLearnQueue(ctx, &storage.StateLearnQueueItem{
				RoomId:               event.RoomID().String(),
				AtEventId:            event.EventID(),
				ViaServer:            string(event.SenderID().ToUserID().Domain()),
				AfterTimestampMillis: time.Now().Add(2 * time.Minute).UnixMilli(), // give the remote server a bit of time to process the event itself
			})
			if err != nil {
				log.Printf("Error queueing state learning of %s at %s: %s", event.RoomID().String(), event.EventID(), err)
			}

			// Cache the fact that we've asked for a state learning update for this room (so we don't keep asking for one).
			// We cache it for longer than the "after timestamp" because it might take a while for the state learning to happen.
			h.stateLearnCache.Set(event.RoomID().String(), true, cache.WithExpiration(5*time.Minute))
		}(h, event)
	}(event, resultCh, waitCh)

	return h.pool.Submit(ctx, event, h, resultCh)
}
