package filter

import (
	"context"
	"time"

	"github.com/matrix-org/policyserv/filter/classification"
)

const StickyEventsFilterName = "StickyEventsFilter"

func init() {
	mustRegister(StickyEventsFilterName, &StickyEventsFilter{})
}

type StickyEventsFilter struct {
}

func (s *StickyEventsFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedStickyEventsFilter{
		set: set,
	}, nil
}

type InstancedStickyEventsFilter struct {
	set *Set
}

func (f *InstancedStickyEventsFilter) Name() string {
	return StickyEventsFilterName
}

func (f *InstancedStickyEventsFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	if input.Event.IsSticky(time.Now(), time.Now()) {
		return []classification.Classification{classification.Spam, classification.Volumetric}, nil
	}
	return nil, nil
}
