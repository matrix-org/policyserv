package filter

import (
	"context"
	"regexp"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
)

const UserIdContainsWordsFilterName = "UserIdContainsWordsFilter"

var findWordsInLocalpartRegex = regexp.MustCompile(`[a-zA-Z0-9]+`)

func init() {
	mustRegister(UserIdContainsWordsFilterName, &UserIdContainsWordsFilter{})
}

type UserIdContainsWordsFilter struct {
}

func (e *UserIdContainsWordsFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedUserIdContainsWordsFilter{
		set:      set,
		maxWords: internal.Dereference(set.communityConfig.UserIdContainsWordsFilterMaxWords),
	}, nil
}

type InstancedUserIdContainsWordsFilter struct {
	set      *Set
	maxWords int
}

func (f *InstancedUserIdContainsWordsFilter) Name() string {
	return UserIdContainsWordsFilterName
}

func (f *InstancedUserIdContainsWordsFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	localpart := input.Event.SenderID().ToUserID().Local()
	words := findWordsInLocalpartRegex.FindAllString(localpart, -1)

	// Remove empty "words" (zero length strings)
	nonEmptyWords := make([]string, 0)
	for _, w := range words {
		if w != "" {
			nonEmptyWords = append(nonEmptyWords, w)
		}
	}

	if len(nonEmptyWords) > f.maxWords {
		return []classification.Classification{classification.Spam}, nil
	}

	return nil, nil
}
