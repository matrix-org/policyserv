package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/ai"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestGptOssSafeguardFilter(t *testing.T) {
	t.Parallel()

	// Note: typically filter tests go as far as creating a whole filter set to ensure they are
	// created appropriately, however because we're unable to realistically spin up a safeguard
	// instance to test against, we just test that the filter is creating the appropriate provider
	// and instanced executor filter, without calling it.
	//
	// This test ultimately gives us confidence that the filter will use the executor and therefore
	// won't randomly be turned on in rooms.

	cnf := &SetConfig{
		InstanceConfig: &config.InstanceConfig{
			GptOssSafeguardAllowedRoomIds: []string{"!allowed:example.org"},
		},
		CommunityConfig: &config.CommunityConfig{
			GptOssSafeguardFilterFailSecure: internal.Pointer(true),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{GptOssSafeguardFilterName},
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
	instanced, ok := set.groups[0].filters[0].(*InstancedAIExecutorFilter[*ai.GptOssSafeguardConfig])
	assert.True(t, ok)
	assert.NotNil(t, instanced)

	// Verify that the config/setup of the executor are carried through correctly. These should have been set during
	// filter creation.
	assert.Equal(t, set, instanced.set)
	assert.Equal(t, GptOssSafeguardFilterName, instanced.Name())
	assert.Equal(t, &ai.GptOssSafeguardConfig{
		FailSecure: true, // should have been set by pulling in the community config above
	}, instanced.config)
	assert.Equal(t, []string{"!allowed:example.org"}, instanced.inRoomIds) // should have been pulled from instance config

	// Verify that the provider is correct. Note that the provider will verify its own config, so we don't need to do
	// that here.
	assert.IsType(t, &ai.GptOssSafeguard{}, instanced.aiProvider)
}
