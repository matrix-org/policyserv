package filter

import (
	"strings"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestLengthFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			// When working on an event, the full event JSON will be used. We need to have a limit that considers
			// both the extra PDU fields and the length of the text body itself for the second set of assertions.
			LengthFilterMaxLength: internal.Pointer(300),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{LengthFilterName},
			RunOnClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything is neutral by default in the test
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
			"body": strings.Repeat("a", 301), // enough to exceed the text body limit above
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "not long enough",
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
	AssertCheckTextAndEvent(t, set, neutralEvent1, harms.NeutralContent())
	AssertCheckEvent(t, set, noopEvent1, harms.NeutralContent()) // text doesn't have a concept of event types, so only check events
}
