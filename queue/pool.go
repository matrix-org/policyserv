package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/media"
	"github.com/matrix-org/policyserv/metrics"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/panjf2000/ants/v2"
	"github.com/prometheus/client_golang/prometheus"
	typedsf "github.com/t2bot/go-typed-singleflight"
)

type PoolResult struct {
	// Nil if there was an error. Otherwise, contains filter results.
	Vectors confidence.Vectors

	// True when the event is considered spam. False indicates neutrality or not-spam.
	// False if there was an error.
	IsProbablySpam bool

	// The error processing the event, if any.
	Err error
}

type sfResult struct {
	firstTimeSeen  bool
	vecs           confidence.Vectors
	isProbablySpam bool
}

type PoolConfig struct {
	ConcurrentPools int
	SizePerPool     int
}

type Pool struct {
	communityManager *community.Manager
	storage          storage.PersistentStorage
	internal         *ants.MultiPool
	sf               *typedsf.Group[*sfResult] // keyed by event ID
}

func NewPool(config *PoolConfig, communityManager *community.Manager, storage storage.PersistentStorage) (*Pool, error) {
	internal, err := ants.NewMultiPool(config.ConcurrentPools, config.SizePerPool, ants.RoundRobin, ants.WithOptions(ants.Options{
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
		return nil, err
	}
	return &Pool{
		communityManager: communityManager,
		storage:          storage,
		internal:         internal,
		sf:               new(typedsf.Group[*sfResult]),
	}, nil
}

// Submit asks the queue to perform filter checking on the given event. If `waitCh` is non-nil, it will be
// called with the result upon completion or error. The `waitCh` is not called if there was a submission
// error - that is instead returned from Submit.
func (p *Pool) Submit(ctx context.Context, event gomatrixserverlib.PDU, mediaDownloader media.Downloader, waitCh chan<- *PoolResult) error {
	metrics.RecordEventCheckRequest(event.RoomID().String())
	t := metrics.StartQueueTimer()

	// Note: waitCh might be nil or unbuffered, so we spawn this in a goroutine later on.
	notifyResult := func(vecs confidence.Vectors, isSpam bool, err error) {
		if err == nil {
			t.ObserveDurationWithExemplar(prometheus.Labels{"waitedUntil": "result"})
		} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			t.ObserveDurationWithExemplar(prometheus.Labels{"waitedUntil": "timeout"})
		} else {
			t.ObserveDurationWithExemplar(prometheus.Labels{"waitedUntil": "error"})
		}

		if waitCh != nil {
			res := &PoolResult{
				Vectors:        vecs,
				IsProbablySpam: isSpam,
				Err:            err,
			}

			// First, check to see if the channel is likely going to be closed already
			if err := ctx.Err(); err != nil {
				log.Printf("[%s | %s] Result channel closed, not sending result (%+v): %s", event.EventID(), event.RoomID().String(), res, err)
				return
			}

			// Consider the context in our delivery of the result
			log.Printf("[%s | %s] Sending result: %+v", event.EventID(), event.RoomID().String(), res)
			select {
			case waitCh <- res:
			case <-ctx.Done():
				log.Printf("[%s | %s] Result channel closed, not sending result (%+v): %s", event.EventID(), event.RoomID().String(), res, ctx.Err())
			}
		}
	}

	workFn := func() {
		// If the context is cancelled, save CPU and don't bother checking
		if err := ctx.Err(); err != nil {
			defer metrics.RecordFailedEventCheck(event.RoomID().String())
			go notifyResult(nil, false, err)
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				log.Printf("Not checking %s because context was cancelled/timed out", event.EventID())
				return
			}
		}

		// Ask the singleflight to do the work (deduplicating scans/work)
		res, err, _ := p.sf.Do(event.EventID(), func() (*sfResult, error) {
			// We create a new context for two reasons:
			// 1. The singleflight might span multiple requests, and we don't want to tie results for all
			//    requests to the first (maybe failed) request.
			// 2. We want to ensure that we continue processing this stuff in the background, even if the
			//    request times out or is cancelled.
			filterCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()

			res, err := p.doFilter(filterCtx, event, mediaDownloader)

			// We do the metrics response within the singleflight so we don't count `firstTimeSeen` multiple times.
			if err != nil {
				defer metrics.RecordFailedEventCheck(event.RoomID().String())
			} else {
				defer metrics.RecordSuccessfulEventCheck(event.RoomID().String(), res.firstTimeSeen, res.vecs)
			}
			return res, err
		})
		log.Printf("[%s | %s] Result from singleflight: %v, %v", event.EventID(), event.RoomID().String(), res, err)
		if res == nil {
			if err == nil {
				// "should never happen"
				err = errors.New("nil result")
			}
			go notifyResult(nil, false, err)
		} else {
			go notifyResult(res.vecs, res.isProbablySpam, err)
		}
	}

	return p.internal.Submit(workFn)
}

func (p *Pool) doFilter(ctx context.Context, event gomatrixserverlib.PDU, mediaDownloader media.Downloader) (*sfResult, error) {
	// First, have we already seen this event?
	res, err := p.storage.GetEventResult(ctx, event.EventID())
	if err != nil {
		return nil, err
	}
	if res != nil {
		return &sfResult{
			firstTimeSeen:  false,
			isProbablySpam: res.IsProbablySpam,
			vecs:           res.ConfidenceVectors,
		}, nil
	}

	// We haven't seen it before. Try to find the community it belongs to.
	set, err := p.communityManager.GetFilterSetForRoomId(ctx, event.RoomID().String())
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, fmt.Errorf("no filter set for room %s (processing %s)", event.RoomID().String(), event.EventID())
	}

	// Run the event through the filters
	vecs, err := set.CheckEvent(ctx, event, mediaDownloader)
	if err != nil {
		return nil, err
	}

	isSpam := set.IsSpamResponse(ctx, vecs)
	if isSpam {
		log.Printf("%s in %s is spam", event.EventID(), event.RoomID().String())
	} else {
		log.Printf("%s in %s is neutral", event.EventID(), event.RoomID().String())
	}

	// Persist results
	err = p.storage.UpsertEventResult(ctx, &storage.StoredEventResult{
		EventId:           event.EventID(),
		IsProbablySpam:    isSpam,
		ConfidenceVectors: vecs,
	})
	if err != nil {
		return nil, err
	}

	// Finally, return
	return &sfResult{
		firstTimeSeen:  true,
		isProbablySpam: isSpam,
		vecs:           vecs,
	}, nil
}
