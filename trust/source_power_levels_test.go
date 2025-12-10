package trust

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestPowerLevelsSource(t *testing.T) {
	t.Parallel()

	db := test.NewMemoryStorage(t)
	source, err := NewPowerLevelsSource(db)
	assert.NoError(t, err)
	assert.NotNil(t, source)

	// No data == no opinion
	res, err := source.HasCapability(context.Background(), "@user:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateDefault, res)

	// Add some data for ease of testing
	stateKey := ""
	err = source.ImportData(context.Background(), "!a:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "m.room.power_levels",
		StateKey: &stateKey,
		Content: map[string]any{
			"state_default": 50,
			"users_default": 0,
			"users": map[string]any{
				"@user:example.org": 100,
			},
		},
	}))
	assert.NoError(t, err)

	// Now check that the user is trusted
	res, err = source.HasCapability(context.Background(), "@user:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateTrue, res)

	// ... and that other users in the same room are not (by returning no opinion)
	res, err = source.HasCapability(context.Background(), "@user:untrusted.example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateDefault, res)

	// ... and that unknown rooms are still untrusted (by returning no opinion)
	res, err = source.HasCapability(context.Background(), "@user:example.org", "!different:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateDefault, res)
}

func TestPowerLevelsSourceRejectsInvalidEvents(t *testing.T) {
	t.Parallel()

	db := test.NewMemoryStorage(t)
	source, err := NewPowerLevelsSource(db)
	assert.NoError(t, err)
	assert.NotNil(t, source)

	// Missing state key
	err = source.ImportData(context.Background(), "!a:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "m.room.power_levels",
		StateKey: nil,
		Content: map[string]any{
			"state_default": 50,
			"users_default": 0,
			"users": map[string]any{
				"@user:example.org": 100,
			},
		},
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a power levels event")

	// Non-empty state key
	stateKey := "foo"
	err = source.ImportData(context.Background(), "!a:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "m.room.power_levels",
		StateKey: &stateKey,
		Content: map[string]any{
			"state_default": 50,
			"users_default": 0,
			"users": map[string]any{
				"@user:example.org": 100,
			},
		},
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a power levels event")

	// Invalid event type
	stateKey = ""
	err = source.ImportData(context.Background(), "!a:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "org.example.wrong_event_type_for_power_levels",
		StateKey: &stateKey,
		Content: map[string]any{
			"state_default": 50,
			"users_default": 0,
			"users": map[string]any{
				"@user:example.org": 100,
			},
		},
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a power levels event")
}
