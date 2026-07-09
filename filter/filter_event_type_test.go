package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestEventTypeFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			EventTypePrefilterAllowedEventTypes:      &[]string{"m.room.message"},
			EventTypePrefilterAllowedStateEventTypes: &[]string{"m.room.topic"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{EventTypeFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything is neutral by default in the test
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	stateKey := "" // deliberately empty to test falsy values

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.topic", // wrong type for allowed non-state events
		Content: map[string]any{
			"body": "body doesn't matter",
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId:  "$spam2",
		RoomId:   "!foo:example.org",
		Type:     "m.room.message", // wrong type for allowed state events
		StateKey: &stateKey,
		Content: map[string]any{
			"body": "body doesn't matter",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message", // correct type for allowed non-state events
		Content: map[string]any{
			"body": "body doesn't matter",
		},
	})
	neutralEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId:  "$neutral2",
		RoomId:   "!foo:example.org",
		Type:     "m.room.topic", // correct type for allowed state events
		StateKey: &stateKey,
		Content: map[string]any{
			"body": "body doesn't matter",
		},
	})

	// This filter emits either Neutral or Allowed, so spam becomes Neutral
	AssertCheckEvent(t, set, spammyEvent1, harms.NeutralContent())
	AssertCheckEvent(t, set, spammyEvent2, harms.NeutralContent())
	AssertCheckEvent(t, set, neutralEvent1, harms.AllowedContent())
	AssertCheckEvent(t, set, neutralEvent2, harms.AllowedContent())
}
