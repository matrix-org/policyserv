package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestKeywordsFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			// Note: "example" is here to ensure the default for KeywordFilterUseFullEvent is false. If it was true,
			// the filter would pick up on the "example.org" in the various IDs.
			KeywordFilterKeywords: &[]string{"spammy spam", "example"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{KeywordFilterName},
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

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "spammy spam",
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "this is a longer example to see if the keywords filter picks up the event",
		},
	})
	neutralEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "this is not spam, nor is it spammy", // deliberately splits the keyword apart to ensure it doesn't count
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
	assertSpamVector(spammyEvent1, true)
	assertSpamVector(spammyEvent2, true)
	assertSpamVector(neutralEvent, false)

	// Also test the text filter implementation
	assertTextSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		body := gjson.Get(string(event.Content()), "body").String()
		vecs, err := set.CheckText(context.Background(), body)
		assert.NoError(t, err)
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
		} else {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
		}
	}
	assertTextSpamVector(spammyEvent1, true)
	assertTextSpamVector(spammyEvent2, true)
	assertTextSpamVector(neutralEvent, false)
}

func TestKeywordsFilterWithFullEvent(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			KeywordFilterKeywords:     &[]string{"spammy spam", "user_id_has_the_keyword_instead"},
			KeywordFilterUseFullEvent: internal.Pointer(true), // this is what we're testing
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{KeywordFilterName},
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

	spammyEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@the_user_id_has_the_keyword_instead_of_the_event_content:example.org",
		Content: map[string]any{
			"body": "not applicable",
		},
	})

	vecs, err := set.CheckEvent(context.Background(), spammyEvent, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
}
