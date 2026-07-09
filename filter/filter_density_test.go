package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestDensityFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			DensityFilterMaxDensity:       internal.Pointer(0.5),
			DensityFilterMinTriggerLength: internal.Pointer(10),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{DensityFilterName},
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

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "aaaaaaaaaaaaaaaaaaaaaaaa", // ~1.0000 density
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "a b c d e f", // ~0.5454 density
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "a          b", // ~0.1666 density
		},
	})
	neutralEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "  a  b  c  d  e  f  g  h  ", // ~0.3076 density
		},
	})
	noopEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.wrong_event_type_for_filter",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	noopEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "2short",
		},
	})

	AssertCheckTextAndEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamFlooding))
	AssertCheckTextAndEvent(t, set, spammyEvent2, harms.ProhibitedContent(harms.SpamFlooding))
	AssertCheckTextAndEvent(t, set, neutralEvent1, harms.NeutralContent())
	AssertCheckTextAndEvent(t, set, neutralEvent2, harms.NeutralContent())
	AssertCheckEvent(t, set, noopEvent1, harms.NeutralContent()) // text doesn't have a concept of event types, so only check events
	AssertCheckTextAndEvent(t, set, noopEvent2, harms.NeutralContent())
}
