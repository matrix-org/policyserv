package filter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/frequency"
	"github.com/matrix-org/policyserv/internal"
)

const FrequencyFilterName = "FrequencyFilter"

func init() {
	mustRegister(FrequencyFilterName, &FrequencyFilter{})
}

type FrequencyFilter struct {
}

func (f *FrequencyFilter) MakeFor(set *Set) (Instanced, error) {
	// The rate limit is set per second over a one minute window, so capture one minute of data.
	counter, err := frequency.NewCounter(set.pubsub, fmt.Sprintf("ff.%s", set.communityId), 60*time.Second)
	if err != nil {
		return nil, err
	}
	return &InstancedFrequencyFilter{
		set:        set,
		counter:    counter,
		eventTypes: internal.Dereference(set.communityConfig.FrequencyFilterEventTypes),
		rateLimit:  internal.Dereference(set.communityConfig.FrequencyFilterRateLimit),
	}, nil
}

type InstancedFrequencyFilter struct {
	set        *Set
	counter    *frequency.Counter
	eventTypes []string
	rateLimit  float64
}

func (f *InstancedFrequencyFilter) Name() string {
	return FrequencyFilterName
}

func (f *InstancedFrequencyFilter) Close() error {
	return f.counter.Close()
}

func (f *InstancedFrequencyFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	doProcess := false
	for _, t := range f.eventTypes {
		if t == input.Event.Type() {
			doProcess = true
			break
		}
	}
	if !doProcess {
		return nil, nil // no opinion
	}

	// First, increment the counter for this user
	err := f.counter.Increment(string(input.Event.SenderID()))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to increment event count for user %s", input.Event.SenderID()), err)
	}

	// Then, figure out if they exceed the rate limit (adding 1 to account for the current event)
	eventsLastMinute, err := f.counter.Get(string(input.Event.SenderID()))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to get event count for user %s", input.Event.SenderID()), err)
	}
	rate := float64(eventsLastMinute+1) / float64(60)
	log.Printf("[%s | %s] Rate for user %s is %f (limit: %f)", input.Event.EventID(), input.Event.RoomID().String(), input.Event.SenderID(), rate, f.rateLimit)
	if rate > f.rateLimit {
		return []classification.Classification{
			classification.Spam,
			classification.Frequency,
		}, nil
	} else {
		return nil, nil
	}
}
