package filter

import (
	"context"

	"github.com/matrix-org/policyserv/ai"
	"github.com/matrix-org/policyserv/filter/condition"
	"github.com/matrix-org/policyserv/harms"
)

type InstancedAIExecutorFilter[ConfigT any] struct {
	name       string
	set        *Set
	config     ConfigT
	aiProvider ai.Provider[ConfigT]
}

func NewInstancedAIExecutorFilter[ConfigT any](name string, set *Set, config ConfigT, aiProvider ai.Provider[ConfigT], inRoomIds []string) InstancedEventFilter {
	instanced := &InstancedAIExecutorFilter[ConfigT]{
		name:       name,
		set:        set,
		config:     config,
		aiProvider: aiProvider,
	}
	return NewConditionalFilter(set, instanced, condition.AnyIn(condition.RoomId, inRoomIds))
}

func (f *InstancedAIExecutorFilter[ConfigT]) Name() string {
	return f.name
}

func (f *InstancedAIExecutorFilter[ConfigT]) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	return f.aiProvider.CheckEvent(ctx, f.config, &ai.Input{
		Event:  input.Event,
		Medias: input.Medias,
	})
}
