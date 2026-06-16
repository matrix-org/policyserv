package filter

import (
	"context"

	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
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
		allowedEventTypes:      internal.Dereference(set.communityConfig.EventTypePrefilterAllowedEventTypes),
		allowedStateEventTypes: internal.Dereference(set.communityConfig.EventTypePrefilterAllowedStateEventTypes),
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

func (f *InstancedEventTypeFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	eventTypeSet := f.allowedEventTypes
	if input.Event.StateKey() != nil {
		eventTypeSet = f.allowedStateEventTypes
	}

	for _, allowedType := range eventTypeSet {
		if allowedType == input.Event.Type() {
			return harms.AllowedContent(), nil
		}
	}

	return harms.NeutralContent(), nil // no opinions when allow-listed
}
