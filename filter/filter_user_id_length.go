package filter

import (
	"context"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
)

const UserIdLengthFilterName = "UserIdLengthFilter"

func init() {
	mustRegister(UserIdLengthFilterName, &UserIdLengthFilter{})
}

type UserIdLengthFilter struct {
}

func (l *UserIdLengthFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedUserIdLengthFilter{
		set:       set,
		maxLength: internal.Dereference(set.communityConfig.UserIdLengthFilterMaxLength),
	}, nil
}

type InstancedUserIdLengthFilter struct {
	set       *Set
	maxLength int
}

func (f *InstancedUserIdLengthFilter) Name() string {
	return UserIdLengthFilterName
}

func (f *InstancedUserIdLengthFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	if len(input.Event.SenderID()) > f.maxLength {
		return []classification.Classification{classification.Spam}, nil
	}
	return nil, nil
}
