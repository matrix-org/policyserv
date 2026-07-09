package filter

import (
	"testing"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
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
			EnabledNames:          []string{StickyEventsFilterName},
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

	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.OtherGeneral))
	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
	AssertCheckEvent(t, set, neutralEvent2, harms.NeutralContent())
}
