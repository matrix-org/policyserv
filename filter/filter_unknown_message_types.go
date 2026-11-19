package filter

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
)

const UnknownMessageTypesFilterName = "UnknownMessageTypesFilter"

func init() {
	mustRegister(UnknownMessageTypesFilterName, &UnknownMessageTypesFilter{})
}

type UnknownMessageTypesFilter struct {
}

func (l *UnknownMessageTypesFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedUnknownMessageTypesFilter{
		set:               set,
		knownMessageTypes: internal.Dereference(set.communityConfig.KnownMessageTypes),
	}, nil
}

type InstancedUnknownMessageTypesFilter struct {
	set               *Set
	knownMessageTypes []string
}

func (f *InstancedUnknownMessageTypesFilter) Name() string {
	return UnknownMessageTypesFilterName
}

func (f *InstancedUnknownMessageTypesFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	if input.Event.Type() != "m.room.message" {
		// not an event we're interested in
		return harms.NeutralContent(), nil
	}

	if input.Event.Redacted() {
		// We can't inspect the message type if it's not there
		return harms.NeutralContent(), nil
	}

	content := &msgtypeOnly{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		return nil, err
	}

	if !slices.Contains(f.knownMessageTypes, content.Msgtype) {
		return harms.ProhibitedContent(harms.PolicyservSpecNonCompliance), nil
	}
	return harms.NeutralContent(), nil
}
