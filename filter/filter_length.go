package filter

import (
	"context"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
)

const LengthFilterName = "LengthFilter"

func init() {
	mustRegister(LengthFilterName, &LengthFilter{})
}

type LengthFilter struct {
}

func (l *LengthFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedLengthFilter{
		set:       set,
		maxLength: internal.Dereference(set.communityConfig.LengthFilterMaxLength),
	}, nil
}

type InstancedLengthFilter struct {
	set       *Set
	maxLength int
}

func (f *InstancedLengthFilter) Name() string {
	return LengthFilterName
}

func (f *InstancedLengthFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	if input.Event.Type() != "m.room.message" {
		// not an event we're interested in
		return nil, nil
	}

	b := input.Event.JSON()
	if len(b) > f.maxLength {
		return []classification.Classification{
			classification.Spam,
			classification.Volumetric,
		}, nil
	}

	return nil, nil
}
