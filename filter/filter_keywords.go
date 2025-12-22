package filter

import (
	"context"
	"strings"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
)

const KeywordFilterName = "KeywordFilter"

func init() {
	mustRegister(KeywordFilterName, &KeywordFilter{})
}

type KeywordFilter struct {
}

func (k *KeywordFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedKeywordFilter{
		set:      set,
		keywords: internal.Dereference(set.communityConfig.KeywordFilterKeywords),
	}, nil
}

type InstancedKeywordFilter struct {
	set      *Set
	keywords []string
}

func (f *InstancedKeywordFilter) Name() string {
	return KeywordFilterName
}

func (f *InstancedKeywordFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	for _, k := range f.keywords {
		if strings.Contains(string(input.Event.Content()), k) {
			return []classification.Classification{classification.Spam}, nil
		}
	}

	return nil, nil
}
