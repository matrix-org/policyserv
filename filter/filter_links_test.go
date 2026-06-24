package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestLinkFilter(t *testing.T) {
	t.Parallel()

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			LinkFilterAllowedUrlGlobs: &[]string{"https://allowed.example.org/*", "https://also-allowed.example.org/*"},
			LinkFilterDeniedUrlGlobs:  &[]string{"https://allowed.example.org/blocked/*"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{LinkFilterName},
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

	// Event with allowed URL
	allowedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$allowed1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "Check this out: https://allowed.example.org/page",
		},
	})

	// Event with a denied URL (path is blocked)
	deniedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$denied1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "https://allowed.example.org/blocked/page",
		},
	})

	// Event with a URL not on the allowed list
	notAllowedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$notallowed1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "See: https://other.example.org/page",
		},
	})

	// Event with no URLs
	noUrlEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$nourl1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "Hello world, no links here!",
		},
	})

	// Event with some allowed and some denied URLs
	mixedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$mixed1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "https://allowed.example.org/ok is allowed but https://other.example.org/path is not allowed",
		},
	})

	AssertCheckTextAndEvent(t, set, allowedEvent, harms.NeutralContent())
	AssertCheckTextAndEvent(t, set, deniedEvent, harms.ProhibitedContent(harms.SpamGeneral)) // deny wins over allow
	AssertCheckTextAndEvent(t, set, notAllowedEvent, harms.ProhibitedContent(harms.SpamGeneral))
	AssertCheckTextAndEvent(t, set, noUrlEvent, harms.NeutralContent())
	AssertCheckTextAndEvent(t, set, mixedEvent, harms.ProhibitedContent(harms.SpamGeneral)) // contains a default-denied URL.
}

func TestLinkFilterDenyListOnly(t *testing.T) {
	t.Parallel()

	// Test with only deny list configured
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			LinkFilterDeniedUrlGlobs: &[]string{"*denied.example.org/path*"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{LinkFilterName},
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

	deniedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$denied1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "http://denied.example.org/path", // we're using http instead of https intentionally to ensure we pick up the scheme
		},
	})

	allowedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$allowed1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "https://denied.example.org/another/page",
		},
	})

	AssertCheckTextAndEvent(t, set, deniedEvent, harms.ProhibitedContent(harms.SpamGeneral))
	AssertCheckTextAndEvent(t, set, allowedEvent, harms.NeutralContent())
}
