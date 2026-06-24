package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestSenderFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			SenderPrefilterAllowedSenders: &[]string{"@alice:example.org"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{SenderFilterName},
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
		Sender:  "@bob:example.org",
		Content: map[string]any{
			"body": "body doesn't matter",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body": "body doesn't matter",
		},
	})

	AssertCheckEvent(t, set, spammyEvent1, harms.NeutralContent()) // this filter actually emits Allowed or Neutral, so spam is Neutral
	AssertCheckEvent(t, set, neutralEvent1, harms.AllowedContent())
}
