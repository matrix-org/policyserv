package filter

import (
	"bytes"
	"context"
	"encoding/csv"
	"log"
	"strings"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/pubsub"
)

const HellbanPrefilterName = "HellbanPrefilter"
const HellbanPostfilterName = "HellbanPostfilter"

func init() {
	mustRegister(HellbanPrefilterName, &HellbanPrefilter{})
	mustRegister(HellbanPostfilterName, &HellbanPostfilter{})
}

type HellbanPrefilter struct {
}

func (h *HellbanPrefilter) MakeFor(set *Set) (Instanced, error) {
	return newPrefilterHellban(set, time.Duration(set.communityConfig.HellbanPostfilterMinutes)*time.Minute)
}

type HellbanPostfilter struct {
}

func (h *HellbanPostfilter) MakeFor(set *Set) (Instanced, error) {
	return newPostfilterHellban(set)
}

type InstancedHellbanFilter struct {
	set *Set

	// If userIdsCache is set, the instanced filter will run as a prefilter. If it's nil, the filter will be a
	// postfilter (checks to see if an event is spam so far, then hellbans as needed).
	userIdsCache *cache.Cache[string, bool] // we don't really use the boolean value, but need to specify it
	forTime      time.Duration

	unsubscribeFn func() error
}

func newPrefilterHellban(set *Set, forTime time.Duration) (*InstancedHellbanFilter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ch, err := set.pubsub.Subscribe(ctx, pubsub.TopicHellban)
	if err != nil {
		return nil, err
	}
	f := &InstancedHellbanFilter{
		set: set,
		userIdsCache: cache.New[string, bool](
			cache.WithJanitorInterval[string, bool](forTime),
		),
		forTime: forTime,
		unsubscribeFn: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			return set.pubsub.Unsubscribe(ctx, ch)
		},
	}
	go func(ch <-chan string, f *InstancedHellbanFilter) {
		for {
			select {
			case encoded := <-ch:
				if encoded == pubsub.ClosingValue {
					log.Println("Closing hellban cache listener")
					break
				}
				if encoded == "" { // sometimes when closing we also get an empty string over the channel
					continue
				}

				communityId, userId := mustDecodeHellban(encoded)
				if communityId != f.set.communityId {
					continue
				}

				// Double check that the user isn't already hellbanned, otherwise we'll effectively permaban
				// them when they try to resend events too early.
				if _, ok := f.userIdsCache.Get(userId); !ok {
					f.userIdsCache.Set(userId, true, cache.WithExpiration(f.forTime))
					log.Printf("Added hellbanned user '%s' to cache", userId)
				}
			}
		}
	}(ch, f)
	return f, nil
}

func newPostfilterHellban(set *Set) (*InstancedHellbanFilter, error) {
	return &InstancedHellbanFilter{
		set: set,
	}, nil
}

func (f *InstancedHellbanFilter) Name() string {
	mode := HellbanPrefilterName
	if f.userIdsCache == nil {
		mode = HellbanPostfilterName
	}
	return mode
}

func (f *InstancedHellbanFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	mode := f.Name()

	eventId := input.Event.EventID()
	roomId := input.Event.RoomID().String()
	log.Printf("[%s | %s] Checking in mode %s", eventId, roomId, mode)

	senderUserId := string(input.Event.SenderID())
	if mode == HellbanPrefilterName {
		if _, ok := f.userIdsCache.Get(senderUserId); ok {
			log.Printf("[%s | %s | %s] Sender '%s' is hellbanned", eventId, roomId, mode, senderUserId)
			return []classification.Classification{
				classification.Spam,
				classification.Frequency,
			}, nil
		}
	} else {
		if f.set.IsSpamResponse(ctx, input.IncrementalConfidenceVectors) {
			log.Printf("[%s | %s | %s] Sender '%s' sent a spammy event", eventId, roomId, mode, senderUserId)
			err := f.set.pubsub.Publish(ctx, pubsub.TopicHellban, mustEncodeHellban(f.set.communityId, senderUserId))
			if err != nil {
				return nil, err
			}
			return []classification.Classification{
				classification.Spam,
				classification.Frequency,
			}, nil
		}
	}

	// If we reached here, the sender isn't spammy by our metrics.
	return nil, nil
}

func (f *InstancedHellbanFilter) Close() error {
	if f.unsubscribeFn == nil {
		return nil
	}

	log.Println("Closing hellban pubsub channel")
	return f.unsubscribeFn()
}

func mustEncodeHellban(communityId string, userId string) string {
	// We use a CSV so we can extend the values in the future if needed, without having to change the
	// topic/channel name. It's also slightly more compact than JSON.

	// This is primarily for tests because not all of them set a community ID on their filter sets.
	if communityId == "" {
		communityId = "default"
	}

	buf := bytes.NewBuffer(nil)
	w := csv.NewWriter(buf)
	err := w.Write([]string{communityId, userId})
	if err != nil {
		panic(err)
	}
	w.Flush()
	return buf.String()
}

func mustDecodeHellban(value string) (string, string) {
	row, err := csv.NewReader(strings.NewReader(value)).Read()
	if err != nil {
		panic(err)
	}
	if len(row) < 2 {
		panic("hellban decode failed: expected at least two values, got: " + value)
	}
	return row[0], row[1]
}
