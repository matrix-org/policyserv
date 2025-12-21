package filter

import (
	"context"
	"encoding/json"
	"log"
	"regexp"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
)

const DensityFilterName = "DensityFilter"

var whitespaceRegex = regexp.MustCompile("\\s")

func init() {
	mustRegister(DensityFilterName, &DensityFilter{})
}

type DensityFilter struct {
}

func (d *DensityFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedDensityFilter{
		set:              set,
		maxDensity:       internal.Dereference(set.communityConfig.DensityFilterMaxDensity),
		minTriggerLength: internal.Dereference(set.communityConfig.DensityFilterMinTriggerLength),
	}, nil
}

type InstancedDensityFilter struct {
	set              *Set
	maxDensity       float64
	minTriggerLength int
}

func (f *InstancedDensityFilter) Name() string {
	return DensityFilterName
}

func (f *InstancedDensityFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	eventId := input.Event.EventID()
	roomId := input.Event.RoomID().String()

	if input.Event.Type() != "m.room.message" {
		// no-op and return the same vectors
		return nil, nil
	}

	content := &bodyOnly{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		// Probably not a string
		return nil, err
	}

	if len(content.Body) < f.minTriggerLength {
		// no-op
		return nil, nil
	}

	beforeTrim := float64(len(content.Body))
	afterTrim := float64(len(whitespaceRegex.ReplaceAllString(content.Body, "")))
	density := afterTrim / beforeTrim

	log.Printf("[%s | %s] Density is %f", eventId, roomId, density)

	if density >= f.maxDensity {
		return []classification.Classification{
			classification.Spam,
			classification.Volumetric,
		}, nil
	}

	return nil, nil
}

type bodyOnly struct {
	Body string `json:"body"`
}
