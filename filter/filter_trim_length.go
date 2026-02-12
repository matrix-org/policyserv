package filter

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/matrix-org/policyserv/filter/classification"
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

func (f *InstancedTrimLengthFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	// Return early on non-message events
	if input.Event.Type() != "m.room.message" {
		return nil, nil
	}

	content := &bodyOnly{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		return nil, err
	}

	return f.CheckText(ctx, content.Body)
}

func (f *InstancedTrimLengthFilter) CheckText(ctx context.Context, text string) ([]classification.Classification, error) {
	beforeTrim := len(text)
	afterTrim := len(strings.TrimSpace(text))

	if (beforeTrim - afterTrim) >= f.maxDifference {
		return []classification.Classification{
			classification.Spam,
			classification.Volumetric,
		}, nil
	}

	return nil, nil
}
