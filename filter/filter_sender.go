package filter

import (
	"context"

	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
)

const SenderFilterName = "SenderFilter"

func init() {
	mustRegister(SenderFilterName, &SenderFilter{})
}

type SenderFilter struct {
}

func (s *SenderFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedSenderFilter{
		set:            set,
		allowedUserIds: internal.Dereference(set.communityConfig.SenderPrefilterAllowedSenders),
	}, nil
}

type InstancedSenderFilter struct {
	set            *Set
	allowedUserIds []string
}

func (f *InstancedSenderFilter) Name() string {
	return SenderFilterName
}

func (f *InstancedSenderFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	for _, s := range f.allowedUserIds {
		if s == string(input.Event.SenderID()) {
			return harms.AllowedContent(), nil
		}
	}

	return harms.NeutralContent(), nil // no opinions when not allow-listed
}
