package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestTrimLengthFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			TrimLengthFilterMaxDifference: internal.Pointer(5),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{TrimLengthFilterName},
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
			"body": "   spaces    ",
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "\t\t\ttabs\t\t\t",
		},
	})
	spammyEvent3 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam3",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "\n\n\nnewlines\n\n\n",
		},
	})
	spammyEvent4 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam3",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "\n\t mixed\t\n ",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "\n 4 total is fine\t\t",
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

	AssertCheckTextAndEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamFlooding))
	AssertCheckTextAndEvent(t, set, spammyEvent2, harms.ProhibitedContent(harms.SpamFlooding))
	AssertCheckTextAndEvent(t, set, spammyEvent3, harms.ProhibitedContent(harms.SpamFlooding))
	AssertCheckTextAndEvent(t, set, spammyEvent4, harms.ProhibitedContent(harms.SpamFlooding))
	AssertCheckTextAndEvent(t, set, neutralEvent1, harms.NeutralContent())
	AssertCheckEvent(t, set, noopEvent1, harms.NeutralContent()) // text doesn't have a concept of event types, so only check events
}
