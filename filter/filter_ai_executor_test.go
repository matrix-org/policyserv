package filter

import (
	"context"
	"errors"
	"testing"

	"github.com/matrix-org/policyserv/ai"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

type arbitraryConfig struct {
	SomeVal bool
}

type TestAIProvider[ConfigT any] struct {
	// Implements ai.Provider[ConfigT]

	T              *testing.T
	Called         bool
	ExpectedConfig ConfigT
	Return         []classification.Classification
	ReturnErr      error
}

func (p *TestAIProvider[ConfigT]) CheckEvent(ctx context.Context, cnf ConfigT, input *ai.Input) ([]classification.Classification, error) {
	assert.NotNil(p.T, ctx, "context is required")
	assert.NotNil(p.T, cnf, "config is required")
	assert.NotNil(p.T, input, "input is required")

	assert.Equal(p.T, p.ExpectedConfig, cnf)

	p.Called = true
	return p.Return, p.ReturnErr
}

func TestInstancedAIExecutorFilter(t *testing.T) {
	t.Parallel()

	// Here we're aiming to test that the instanced filter properly calls the provider with what it's supposed to. Other
	// tests cover whether it gets created properly in-context (ie: whether the OpenAI Omni filter creates the right
	// provider & instanced filter).

	ctx := context.Background()
	name := "TestAIExecutor"
	allowedRoomId := "!allowed:example.org"
	// the actual config type doesn't really matter - we just want to make sure it gets passed down appropriately
	provider := &TestAIProvider[*arbitraryConfig]{
		T:              t,
		ExpectedConfig: &arbitraryConfig{SomeVal: true},
	}
	set := &Set{
		communityConfig: &config.CommunityConfig{},
	}
	instance := NewInstancedAIExecutorFilter(name, set, provider.ExpectedConfig, provider, []string{allowedRoomId})
	assert.NotNil(t, instance)
	assert.Equal(t, name, instance.Name()) // it should carry the name we gave it

	// Ensure the AI Provider isn't called for rooms it's not supposed to
	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!not_the_right_room_for_this_filter:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":    "hello world",
			"msgtype": "m.text",
		},
	})
	vecs, err := instance.CheckEvent(ctx, &Input{Event: event})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(vecs))
	assert.False(t, provider.Called)

	// Ensure the AI Provider is called for rooms it's supposed to
	event = test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  allowedRoomId,
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body":    "hello world",
			"msgtype": "m.text",
		},
	})
	vecs, err = instance.CheckEvent(ctx, &Input{Event: event})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(vecs))
	assert.True(t, provider.Called)

	// Ensure errors are passed through
	retErr := errors.New("test error")
	provider.ReturnErr = retErr
	_, err = instance.CheckEvent(ctx, &Input{Event: event})
	assert.Equal(t, retErr, err)

	// Ensure classifications are passed through
	ret := []classification.Classification{classification.Spam, classification.Volumetric}
	provider.Return = ret
	provider.ReturnErr = nil
	vecs, err = instance.CheckEvent(ctx, &Input{Event: event})
	assert.NoError(t, err)
	assert.Equal(t, ret, vecs)
}
