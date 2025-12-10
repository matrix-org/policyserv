package filter

import (
	"context"
	"errors"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

type TestAIProvider struct {
	T         *testing.T
	Called    bool
	Return    []classification.Classification
	ReturnErr error
}

func (p *TestAIProvider) CheckEvent(ctx context.Context, cnf *aiFilterConfig, input *Input) ([]classification.Classification, error) {
	assert.NotNil(p.T, ctx, "context is required")
	assert.NotNil(p.T, cnf, "config is required")
	assert.NotNil(p.T, input, "input is required")

	assert.Equal(p.T, true, cnf.FailSecure)

	p.Called = true
	return p.Return, p.ReturnErr
}

func TestOpenAIFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Note: typically filter tests go as far as creating a whole filter set to ensure they are
	// created appropriately, however because we're unable to realistically test against OpenAI
	// directly, we just test that the filter is calling a function properly. This means we create
	// the instanced filter structure manually instead of letting the filter set do it (as letting
	// the set do it would require a real OpenAI API key).

	allowedRoomId := "!allowed:example.org"
	aiProvider := &TestAIProvider{
		T: t,
	}
	instance := &InstancedOpenAIFilter{
		set: &Set{ // the filter doesn't use most of this
			communityConfig: &config.CommunityConfig{
				OpenAIFilterFailSecure: true, // asserted in TestAIProvider
			},
		},
		aiProvider: aiProvider,
		inRoomIds:  []string{allowedRoomId},
	}

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
	assert.False(t, aiProvider.Called)

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
	assert.True(t, aiProvider.Called)

	// Ensure errors are passed through
	retErr := errors.New("test error")
	aiProvider.ReturnErr = retErr
	_, err = instance.CheckEvent(ctx, &Input{Event: event})
	assert.Equal(t, retErr, err)

	// Ensure classifications are passed through
	ret := []classification.Classification{classification.Spam, classification.Volumetric}
	aiProvider.Return = ret
	aiProvider.ReturnErr = nil
	vecs, err = instance.CheckEvent(ctx, &Input{Event: event})
	assert.NoError(t, err)
	assert.Equal(t, ret, vecs)
}
