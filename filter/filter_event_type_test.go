package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
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
			EnabledNames:           []string{EventTypeFilterName},
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

	assertSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		vecs, err := set.CheckEvent(context.Background(), event, nil)
		assert.NoError(t, err)
		if isSpam {
			// Because the filter doesn't flag things as "spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
		} else {
			assert.Equal(t, 0.0, vecs.GetVector(classification.Spam))
		}
	}
	assertSpamVector(spammyEvent1, true)
	assertSpamVector(spammyEvent2, true)
	assertSpamVector(neutralEvent1, false)
	assertSpamVector(neutralEvent2, false)
}
