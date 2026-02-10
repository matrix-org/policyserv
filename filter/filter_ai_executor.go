package filter

import (
	"context"
	"slices"

	"github.com/matrix-org/policyserv/ai"
	"github.com/matrix-org/policyserv/filter/classification"
)

type InstancedAIExecutorFilter[ConfigT any] struct {
	name       string
	set        *Set
	config     ConfigT
	aiProvider ai.Provider[ConfigT]
	inRoomIds  []string
}

func NewInstancedAIExecutorFilter[ConfigT any](name string, set *Set, config ConfigT, aiProvider ai.Provider[ConfigT], inRoomIds []string) *InstancedAIExecutorFilter[ConfigT] {
	return &InstancedAIExecutorFilter[ConfigT]{
		name:       name,
		set:        set,
		config:     config,
		aiProvider: aiProvider,
		inRoomIds:  inRoomIds,
	}
}

func (f *InstancedAIExecutorFilter[ConfigT]) Name() string {
	return f.name
}

func (f *InstancedAIExecutorFilter[ConfigT]) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	if !slices.Contains(f.inRoomIds, input.Event.RoomID().String()) {
		return nil, nil // this filter isn't allowed to execute in here
	}
	return f.aiProvider.CheckEvent(ctx, f.config, &ai.Input{
		Event:  input.Event,
		Medias: input.Medias,
	})
}
