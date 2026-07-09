package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestKeywordsFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			// Note: "example" is here to ensure the default for KeywordFilterUseFullEvent is false. If it was true,
			// the filter would pick up on the "example.org" in the various IDs.
			KeywordFilterKeywords: &[]string{"spammy spam", "example"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{KeywordFilterName},
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

	AssertCheckTextAndEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamGeneral))
	AssertCheckTextAndEvent(t, set, spammyEvent2, harms.ProhibitedContent(harms.SpamGeneral))
	AssertCheckTextAndEvent(t, set, neutralEvent, harms.NeutralContent())
}

func TestKeywordsFilterWithFullEvent(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			KeywordFilterKeywords:     &[]string{"spammy spam", "user_id_has_the_keyword_instead"},
			KeywordFilterUseFullEvent: internal.Pointer(true), // this is what we're testing
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{KeywordFilterName},
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

	spammyEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@the_user_id_has_the_keyword_instead_of_the_event_content:example.org",
		Content: map[string]any{
			"body": "not applicable",
		},
	})

	AssertCheckEvent(t, set, spammyEvent, harms.ProhibitedContent(harms.SpamGeneral))
}
