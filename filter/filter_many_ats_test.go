package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestManyAtsFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			ManyAtsFilterMaxAts: internal.Pointer(2),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{ManyAtsFilterName},
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
			"body": "@_@", // 2 ats (enough to trip the text test)
			"m.mentions": []string{ // 3 ats
				"@user1:example.org",
				"@user2:example.org",
				"@user3:example.org",
			},
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "test m.mentions",
			"m.mentions": []string{ // 3 ats
				"@user1:example.org",
				"@user2:example.org",
				"@user3:example.org",
			},
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "one @", // 1 is less than 2
		},
	})

	AssertCheckTextAndEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamFlooding))
	AssertCheckEvent(t, set, spammyEvent2, harms.ProhibitedContent(harms.SpamFlooding)) // `m.mentions` isn't checked by text, so check event only
	AssertCheckTextAndEvent(t, set, neutralEvent1, harms.NeutralContent())
}
