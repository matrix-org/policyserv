package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/ai"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestOpenAIOmniFilter(t *testing.T) {
	t.Parallel()

	// Note: typically filter tests go as far as creating a whole filter set to ensure they are
	// created appropriately, however because we're unable to realistically test against OpenAI
	// directly, we just test that the filter is creating the appropriate provider and instanced
	// executor filter, without calling it.
	//
	// This test ultimately gives us confidence that the filter will use the executor and therefore
	// won't randomly be turned on in rooms.

	cnf := &SetConfig{
		InstanceConfig: &config.InstanceConfig{
			OpenAIAllowedRoomIds: []string{"!allowed:example.org"},
			OpenAIApiKey:         "not a real key",
		},
		CommunityConfig: &config.CommunityConfig{
			OpenAIFilterFailSecure: internal.Pointer(true),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{OpenAIOmniFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	// Pull the filter out of the set to inspect its type & config.
	// If this for some reason isn't "ok", then the filter created the very wrong thing.
	instanced, ok := set.groups[0].filters[0].(*InstancedAIExecutorFilter[*ai.OpenAIOmniModerationConfig])
	assert.True(t, ok)
	assert.NotNil(t, instanced)

	// Verify that the config/setup of the executor are carried through correctly
	assert.Equal(t, set, instanced.set)                     // should have been set during filter creation
	assert.Equal(t, OpenAIOmniFilterName, instanced.Name()) // should have been set during filter creation
	assert.Equal(t, &ai.OpenAIOmniModerationConfig{
		FailSecure: true, // should have been set by pulling in the community config above
	}, instanced.config)
	assert.Equal(t, []string{"!allowed:example.org"}, instanced.inRoomIds) // should have been pulled from instance config

	// Verify that the provider is correct. Note that the provider verifies it got a non-empty API key, so this also
	// tests that we got *something* resembling an API key as far as the provider. We can't really test that the exact
	// API key from the instance config made it there, but we can be pretty sure. Live testing would reveal whether the
	// correct API key is making it.
	assert.IsType(t, &ai.OpenAIOmniModeration{}, instanced.aiProvider)
}
