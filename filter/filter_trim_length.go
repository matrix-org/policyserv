package filter

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/matrix-org/policyserv/filter/classification"
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
		maxDifference: set.communityConfig.TrimLengthFilterMaxDifference,
	}, nil
}

type InstancedTrimLengthFilter struct {
	set           *Set
	maxDifference int
}

func (f *InstancedTrimLengthFilter) Name() string {
	return TrimLengthFilterName
}

func (f *InstancedTrimLengthFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	// Return early on non-message events
	if input.Event.Type() != "m.room.message" {
		return nil, nil
	}

	content := &bodyOnly{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		return nil, err
	}

	beforeTrim := len(content.Body)
	afterTrim := len(strings.TrimSpace(content.Body))

	if (beforeTrim - afterTrim) >= f.maxDifference {
		return []classification.Classification{
			classification.Spam,
			classification.Volumetric,
		}, nil
	}

	return nil, nil
}
