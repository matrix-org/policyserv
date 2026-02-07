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

func TestLinkFilter(t *testing.T) {
	t.Parallel()

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			LinkFilterAllowedUrlGlobs: &[]string{"https://allowed.example.org/*", "https://also-allowed.example.org/*"},
			LinkFilterDeniedUrlGlobs:  &[]string{"https://allowed.example.org/blocked/*"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{LinkFilterName},
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

	assertSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		vecs, err := set.CheckEvent(context.Background(), event, nil)
		assert.NoError(t, err)
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
		} else {
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
		}
	}

	assertSpamVector(allowedEvent, false)
	assertSpamVector(deniedEvent, true) // deny wins over allow
	assertSpamVector(notAllowedEvent, true)
	assertSpamVector(noUrlEvent, false)
	assertSpamVector(mixedEvent, true) //contains a default-denied URL.
}

func TestLinkFilterDenyListOnly(t *testing.T) {
	t.Parallel()

	// Test with only deny list configured
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			LinkFilterDeniedUrlGlobs: &[]string{"*denied.example.org/path*"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{LinkFilterName},
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

	deniedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$denied1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "http://denied.example.org/path",  // we're using http instead of https intentionally to ensure we pick up the scheme
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

	vecs, err := set.CheckEvent(context.Background(), deniedEvent, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))

	vecs, err = set.CheckEvent(context.Background(), allowedEvent, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
}