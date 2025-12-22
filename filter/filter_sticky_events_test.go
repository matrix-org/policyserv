package filter

import (
	"context"
	"testing"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestStickyEventsFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			StickyEventsFilterAllowStickyEvents: internal.Pointer(false),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{StickyEventsFilterName},
			MinimumSpamVectorValue: 0,
			MaximumSpamVectorValue: 1,
		}},
	}

	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	now := time.Now()

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "doesn't matter",
		},
		StickyUntil: now.Add(1 * time.Hour), // sticky for the entire duration of our check, so spammy
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "doesn't matter",
		},

		// No sticky until time means the event should be neutral ("not sticky")
	})
	neutralEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "doesn't matter",
		},
		StickyUntil: time.Now().Add(-1 * time.Hour), // sticky a long time ago, but not anymore, so neutral
	})

	assertSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		vecs, err := set.CheckEvent(context.Background(), event, nil)
		assert.NoError(t, err)
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
			assert.Equal(t, 1.0, vecs.GetVector(classification.Volumetric))
		} else {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
			assert.Equal(t, 0.0, vecs.GetVector(classification.Volumetric))
		}
	}
	assertSpamVector(spammyEvent1, true)
	assertSpamVector(neutralEvent1, false)
	assertSpamVector(neutralEvent2, false)
}
