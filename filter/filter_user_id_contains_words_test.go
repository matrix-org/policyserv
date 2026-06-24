package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestUserIdContainsWordsFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			UserIdContainsWordsFilterMaxWords: internal.Pointer(3),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{UserIdContainsWordsFilterName},
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
		Type:    "org.example.event_type_does_not_matter",
		Sender:  "@user.with_four-words:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.event_type_does_not_matter",
		Sender:  "@two.words:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	neutralEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral2",
		RoomId:  "!foo:example.org",
		Type:    "org.example.event_type_does_not_matter",
		Sender:  "@-_-empty_words.-.:example.org", // zero length words shouldn't count
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})

	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamFlooding))
	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
	AssertCheckEvent(t, set, neutralEvent2, harms.NeutralContent())
}
