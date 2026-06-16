package homeserver

import (
	"context"
	"errors"
	"log"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/queue"
)

func (h *Homeserver) RunFilters(ctx context.Context, event gomatrixserverlib.PDU, waitCh chan<- *queue.PoolResult) error {
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
				ContentInfo: nil,
				Err:         err,
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

		// We'd like to "learn" room state, if we can, so do that. We do this async to avoid blocking the filter request.
		go h.queueLearnStateIfNeeded(ctx, res, event)
	}(event, resultCh, waitCh)

	return h.pool.Submit(ctx, event, h, resultCh)
}
