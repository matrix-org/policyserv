package filter

import (
	"context"
	"strings"

	"github.com/matrix-org/policyserv/harms"
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
		set:          set,
		keywords:     internal.Dereference(set.communityConfig.KeywordFilterKeywords),
		useFullEvent: internal.Dereference(set.communityConfig.KeywordFilterUseFullEvent),
	}, nil
}

type InstancedKeywordFilter struct {
	set          *Set
	keywords     []string
	useFullEvent bool
}

func (f *InstancedKeywordFilter) Name() string {
	return KeywordFilterName
}

func (f *InstancedKeywordFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	toScan := string(input.Event.Content())
	if f.useFullEvent {
		toScan = string(input.Event.JSON())
	}
	return f.CheckText(ctx, toScan)
}

func (f *InstancedKeywordFilter) CheckText(ctx context.Context, text string) (*harms.ContentInfo, error) {
	for _, k := range f.keywords {
		if strings.Contains(text, k) {
			return harms.ProhibitedContent(harms.SpamGeneral), nil
		}
	}

	return harms.NeutralContent(), nil
}
