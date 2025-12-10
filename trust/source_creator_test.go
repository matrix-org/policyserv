package trust

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestCreatorSource(t *testing.T) {
	t.Parallel()

	db := test.NewMemoryStorage(t)
	source, err := NewCreatorSource(db)
	assert.NoError(t, err)
	assert.NotNil(t, source)

	// No data == no opinion
	res, err := source.HasCapability(context.Background(), "@user:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateDefault, res)

	// Add some data for ease of testing
	stateKey := ""
	err = source.ImportData(context.Background(), "!a:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "m.room.create",
		StateKey: &stateKey,
		Sender:   "@user:example.org",
		Content: map[string]any{
			"room_version": "12",
			"additional_creators": []string{
				"@other:example.org",
			},
		},
	}))
	assert.NoError(t, err)

	// Now check that the user is trusted
	res, err = source.HasCapability(context.Background(), "@user:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateTrue, res)
	res, err = source.HasCapability(context.Background(), "@other:example.org", "!a:example.org", CapabilityMedia)
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

func TestCreatorSourceRejectsInvalidEvents(t *testing.T) {
	t.Parallel()

	db := test.NewMemoryStorage(t)
	source, err := NewCreatorSource(db)
	assert.NoError(t, err)
	assert.NotNil(t, source)

	// Missing state key
	err = source.ImportData(context.Background(), "!a:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "m.room.create",
		StateKey: nil,
		Sender:   "@user:example.org",
		Content: map[string]any{
			"room_version": "12",
			"additional_creators": []string{
				"@other:example.org",
			},
		},
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a create event")

	// Non-empty state key
	stateKey := "foo"
	err = source.ImportData(context.Background(), "!a:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "m.room.create",
		StateKey: &stateKey,
		Sender:   "@user:example.org",
		Content: map[string]any{
			"room_version": "12",
			"additional_creators": []string{
				"@other:example.org",
			},
		},
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a create event")

	// Invalid event type
	stateKey = ""
	err = source.ImportData(context.Background(), "!a:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "org.example.wrong_event_type_for_create",
		StateKey: &stateKey,
		Sender:   "@user:example.org",
		Content: map[string]any{
			"room_version": "12",
			"additional_creators": []string{
				"@other:example.org",
			},
		},
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a create event")
}

func TestCreatorSourceIgnoresV12UnderRooms(t *testing.T) {
	t.Parallel()

	db := test.NewMemoryStorage(t)
	source, err := NewCreatorSource(db)
	assert.NoError(t, err)
	assert.NotNil(t, source)

	stateKey := ""
	err = source.ImportData(context.Background(), "!a:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "m.room.create",
		StateKey: &stateKey,
		Sender:   "@user:example.org",
		Content: map[string]any{
			"room_version": "11", // not a v12 room, but still known to GMSL
			"additional_creators": []string{
				"@other:example.org",
			},
		},
	}))
	assert.NoError(t, err)

	// Neither user should be trusted because the room isn't a v12+ room
	res, err := source.HasCapability(context.Background(), "@user:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateDefault, res)
	res, err = source.HasCapability(context.Background(), "@other:example.org", "!a:example.org", CapabilityMedia)
	assert.NoError(t, err)
	assert.Equal(t, TristateDefault, res)

	// Further validate that internally it thinks no one is creator
	creators, err := source.GetCreators(context.Background(), "!a:example.org")
	assert.NoError(t, err)
	assert.Empty(t, creators)
}
