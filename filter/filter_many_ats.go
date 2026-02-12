package filter

import (
	"context"
	"strings"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
)

const ManyAtsFilterName = "ManyAtsFilter"

func init() {
	mustRegister(ManyAtsFilterName, &ManyAtsFilter{})
}

type ManyAtsFilter struct {
}

func (m *ManyAtsFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedManyAtsFilter{
		set:    set,
		maxAts: internal.Dereference(set.communityConfig.ManyAtsFilterMaxAts),
	}, nil
}

type InstancedManyAtsFilter struct {
	set    *Set
	maxAts int
}

func (f *InstancedManyAtsFilter) Name() string {
	return ManyAtsFilterName
}

func (f *InstancedManyAtsFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	return f.CheckText(ctx, string(input.Event.Content()))
}

func (f *InstancedManyAtsFilter) CheckText(ctx context.Context, text string) ([]classification.Classification, error) {
	if strings.Count(text, "@") >= f.maxAts {
		return []classification.Classification{
			classification.Spam,
			classification.Mentions,
		}, nil
	}

	return nil, nil
}
