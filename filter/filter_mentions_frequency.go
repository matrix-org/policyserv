package filter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/frequency"
	"github.com/matrix-org/policyserv/internal"
)

const MentionsFrequencyFilterName = "MentionsFrequencyFilter"

func init() {
	mustRegister(MentionsFrequencyFilterName, &MentionsFrequencyFilter{})
}

type MentionsFrequencyFilter struct {
}

func (f *MentionsFrequencyFilter) MakeFor(set *Set) (Instanced, error) {
	// The rate limit is set per second over a one minute window, so capture one minute of data.
	counter, err := frequency.NewCounter(set.pubsub, fmt.Sprintf("fm.%s", set.communityId), 60*time.Second)
	if err != nil {
		return nil, err
	}
	rateLimit := internal.Dereference(set.communityConfig.MentionFrequencyFilterRateLimit)
	return &InstancedMentionsFrequencyFilter{
		set:       set,
		counter:   counter,
		rateLimit: rateLimit,
		// Dev note: if this ever changes to not use the mentions filter internally, add tests to ensure it counts mentions correctly.
		mentionsFilter: &InstancedMentionsFilter{
			set:           set,
			maxMentions:   int(math.Ceil((rateLimit * 60) + 1)), // plus one to ensure we will automatically exceed the rate limit
			minNameLength: internal.Dereference(set.communityConfig.MentionFrequencyFilterMinPlaintextLength),
		},
	}, nil
}

type InstancedMentionsFrequencyFilter struct {
	set            *Set
	counter        *frequency.Counter
	rateLimit      float64
	mentionsFilter *InstancedMentionsFilter
}

func (f *InstancedMentionsFrequencyFilter) Name() string {
	return MentionsFrequencyFilterName
}

func (f *InstancedMentionsFrequencyFilter) Close() error {
	return f.counter.Close()
}

func (f *InstancedMentionsFrequencyFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	numMentions, err := f.mentionsFilter.CountMentionsToLimit(ctx, input.Event, f.mentionsFilter.maxMentions)
	if err != nil {
		return nil, err
	}

	// We're about to add `numMentions` to the rate limit manually, so capture the current value before incrementing
	// in case the incrementing gets picked up quickly by the counter.
	eventsLastMinute, err := f.counter.Get(string(input.Event.SenderID()))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to get mentions count for user %s", input.Event.SenderID()), err)
	}

	// Increment accordingly
	for i := 0; i < numMentions; i++ {
		err = f.counter.Increment(string(input.Event.SenderID()))
		if err != nil {
			return nil, errors.Join(fmt.Errorf("failed to increment mentions count for user %s", input.Event.SenderID()), err)
		}
	}

	// Then, figure out if they exceed the rate limit
	rate := float64(eventsLastMinute+numMentions) / float64(60)
	log.Printf("[%s | %s] Rate for user %s is %f (limit: %f)", input.Event.EventID(), input.Event.RoomID().String(), input.Event.SenderID(), rate, f.rateLimit)
	if rate > f.rateLimit {
		return []classification.Classification{
			classification.Spam,
			classification.Frequency,
			classification.Mentions,
		}, nil
	} else {
		return nil, nil
	}
}
