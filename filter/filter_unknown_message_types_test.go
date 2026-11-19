package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestUnknownMessageTypesFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			KnownMessageTypes: internal.Pointer([]string{"m.text", "m.emote"}),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{UnknownMessageTypesFilterName},
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
			"msgtype": "org.example.unstable",
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.notice", // m.notice is known in the spec, but not configured as known above
		},
	})
	spammyEvent3 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam3",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"no_msgtype": "this event doesn't have any msgtype, but also isn't redacted",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
		},
	})
	noopEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.wrong_event_type_for_filter",
		Content: map[string]any{
			"msgtype": "doesn't matter",
		},
	})
	noopEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.wrong_event_type_for_filter",
		Content: map[string]any{
			"msgtype": "will be redacted",
		},
	})
	noopEvent2.Redact() // verify that redacted events are not checked

	// This filter doesn't check text, so only check events
	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.PolicyservSpecNonCompliance))
	AssertCheckEvent(t, set, spammyEvent2, harms.ProhibitedContent(harms.PolicyservSpecNonCompliance))
	AssertCheckEvent(t, set, spammyEvent3, harms.ProhibitedContent(harms.PolicyservSpecNonCompliance))
	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
	AssertCheckEvent(t, set, noopEvent1, harms.NeutralContent())
	AssertCheckEvent(t, set, noopEvent2, harms.NeutralContent())
}
