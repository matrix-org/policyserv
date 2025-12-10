package filter

import (
	"context"
	"encoding/json"

	"github.com/matrix-org/policyserv/filter/classification"
)

const MediaFilterName = "MediaFilter"

func init() {
	mustRegister(MediaFilterName, &MediaFilter{})
}

type MediaFilter struct {
}

func (m *MediaFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedMediaFilter{
		set:        set,
		mediaTypes: set.communityConfig.MediaFilterMediaTypes,
	}, nil
}

type InstancedMediaFilter struct {
	set        *Set
	mediaTypes []string
}

func (f *InstancedMediaFilter) Name() string {
	return MediaFilterName
}

func (f *InstancedMediaFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	// Check event type (for stickers) first
	for _, mediaType := range f.mediaTypes {
		if mediaType == input.Event.Type() {
			return []classification.Classification{classification.Spam}, nil
		}
	}

	// the msgtype check only applies to regular room messages, so return early if we can
	if input.Event.Type() != "m.room.message" {
		return nil, nil
	}

	content := &msgtypeOnly{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		// Probably not a string
		return nil, err
	}

	for _, mediaType := range f.mediaTypes {
		if mediaType == content.Msgtype {
			return []classification.Classification{
				classification.Spam,
			}, nil
		}
	}

	return nil, nil
}

type msgtypeOnly struct {
	Msgtype string `json:"msgtype"`
}
