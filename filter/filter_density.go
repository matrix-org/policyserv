package filter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	"github.com/matrix-org/policyserv/harms"
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

func (f *InstancedDensityFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	eventId := input.Event.EventID()
	roomId := input.Event.RoomID().String()

	if input.Event.Type() != "m.room.message" {
		return harms.NeutralContent(), nil
	}

	content := &bodyOnly{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		// Probably not a string
		return nil, err
	}

	return f.checkTextWithLogging(ctx, content.Body, fmt.Sprintf("%s | %s", eventId, roomId))
}

func (f *InstancedDensityFilter) CheckText(ctx context.Context, text string) (*harms.ContentInfo, error) {
	return f.checkTextWithLogging(ctx, text, "CheckText")
}

func (f *InstancedDensityFilter) checkTextWithLogging(ctx context.Context, text string, logPrefix string) (*harms.ContentInfo, error) {
	if len(text) < f.minTriggerLength {
		// no-op
		return harms.NeutralContent(), nil
	}

	beforeTrim := float64(len(text))
	afterTrim := float64(len(whitespaceRegex.ReplaceAllString(text, "")))
	density := afterTrim / beforeTrim

	log.Printf("[%s] Density is %f", logPrefix, density)

	if density >= f.maxDensity {
		return harms.ProhibitedContent(harms.SpamFlooding), nil
	}

	return harms.NeutralContent(), nil
}

type bodyOnly struct {
	Body string `json:"body"`
}
