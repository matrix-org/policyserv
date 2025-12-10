package filter

import (
	"context"

	"github.com/matrix-org/policyserv/filter/classification"
)

const EventTypeFilterName = "EventTypeFilter"

func init() {
	mustRegister(EventTypeFilterName, &EventTypeFilter{})
}

type EventTypeFilter struct {
}

func (e *EventTypeFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedEventTypeFilter{
		set:                    set,
		allowedEventTypes:      set.communityConfig.EventTypePrefilterAllowedEventTypes,
		allowedStateEventTypes: set.communityConfig.EventTypePrefilterAllowedStateEventTypes,
	}, nil
}

type InstancedEventTypeFilter struct {
	set                    *Set
	allowedEventTypes      []string
	allowedStateEventTypes []string
}

func (f *InstancedEventTypeFilter) Name() string {
	return EventTypeFilterName
}

func (f *InstancedEventTypeFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	eventTypeSet := f.allowedEventTypes
	if input.Event.StateKey() != nil {
		eventTypeSet = f.allowedStateEventTypes
	}

	for _, allowedType := range eventTypeSet {
		if allowedType == input.Event.Type() {
			return []classification.Classification{classification.Spam.Invert()}, nil // we expect to be run as a prefilter, so explicitly return not spam
		}
	}

	return nil, nil // no opinions when allow-listed
}
