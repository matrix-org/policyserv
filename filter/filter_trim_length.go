package filter

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
)

const TrimLengthFilterName = "TrimLengthFilter"

func init() {
	mustRegister(TrimLengthFilterName, &TrimLengthFilter{})
}

type TrimLengthFilter struct {
}

func (t *TrimLengthFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedTrimLengthFilter{
		set:           set,
		maxDifference: internal.Dereference(set.communityConfig.TrimLengthFilterMaxDifference),
	}, nil
}

type InstancedTrimLengthFilter struct {
	set           *Set
	maxDifference int
}

func (f *InstancedTrimLengthFilter) Name() string {
	return TrimLengthFilterName
}

func (f *InstancedTrimLengthFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	// Return early on non-message events
	if input.Event.Type() != "m.room.message" {
		return harms.NeutralContent(), nil
	}

	content := &bodyOnly{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		return nil, err
	}

	return f.CheckText(ctx, content.Body)
}

func (f *InstancedTrimLengthFilter) CheckText(ctx context.Context, text string) (*harms.ContentInfo, error) {
	beforeTrim := len(text)
	afterTrim := len(strings.TrimSpace(text))

	if (beforeTrim - afterTrim) >= f.maxDifference {
		return harms.ProhibitedContent(harms.SpamGeneral, harms.SpamFlooding), nil
	}

	return harms.NeutralContent(), nil
}
