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
			LinkFilterAllowedUrlGlobs: &[]string{"https://github.com/*", "https://spec.matrix.org/*"},
			LinkFilterDeniedUrlGlobs:  &[]string{"https://github.com/banned-user/*"},
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

	// Event with allowed URL
	allowedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$allowed1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "Check this out: https://github.com/matrix-org/policyserv",
		},
	})

	// Event with denied URL (banned user repo, even though github.com is allowed)
	deniedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$denied1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "https://github.com/banned-user/repo",
		},
	})

	// Event with URL not on allow list
	notAllowedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$notallowed1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "See: https://nsfw-site.example/bad-stuff",
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

	// Event with multiple URLs (one good, one bad)
	mixedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$mixed1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "Good: https://github.com/foo Bad: https://evil.com/path",
		},
	})

	assertSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		vecs, err := set.CheckEvent(context.Background(), event, nil)
		assert.NoError(t, err)
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
		} else {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
		}
	}

	assertSpamVector(allowedEvent, false)
	assertSpamVector(deniedEvent, true) // deny wins over allow
	assertSpamVector(notAllowedEvent, true)
	assertSpamVector(noUrlEvent, false)
	assertSpamVector(mixedEvent, true) // one bad URL = spam
}

func TestLinkFilterDenyWins(t *testing.T) {
	t.Parallel()

	// This test specifically verifies that deny list takes priority over allow list
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			LinkFilterAllowedUrlGlobs: &[]string{"https://example.com/*"},
			LinkFilterDeniedUrlGlobs:  &[]string{"https://example.com/blocked*"},
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

	// URL matches both allow and deny - deny should win
	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$denywins1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "https://example.com/blocked-page",
		},
	})

	vecs, err := set.CheckEvent(context.Background(), event, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1.0, vecs.GetVector(classification.Spam), "Deny list should take priority over allow list")
}

func TestLinkFilterDenyListOnly(t *testing.T) {
	t.Parallel()

	// Test with only deny list configured
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			LinkFilterDeniedUrlGlobs: &[]string{"*nsfw-site.example*"},
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
			"body":    "https://nsfw-site.example/path",
		},
	})

	allowedEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$allowed1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "https://example.org/page",
		},
	})

	vecs, err := set.CheckEvent(context.Background(), deniedEvent, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))

	vecs, err = set.CheckEvent(context.Background(), allowedEvent, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
}
