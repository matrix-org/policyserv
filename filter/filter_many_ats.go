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

func (f *InstancedManyAtsFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	if strings.Count(string(input.Event.Content()), "@") >= f.maxAts {
		return []classification.Classification{
			classification.Spam,
			classification.Mentions,
		}, nil
	}

	return nil, nil
}
