package filter

import (
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestProtectLocalUserFilter(t *testing.T) {
	t.Parallel()

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		InstanceConfig: &config.InstanceConfig{
			HomeserverName: "example.org",
			JoinLocalpart:  "alice",
		},
		Groups: []*SetGroupConfig{{
			// The filter is force-enabled in the community manager, which we need to mimic here
			EnabledNames:          []string{ProtectLocalUserFilterName},
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
		EventId:  "$spam1",
		RoomId:   "!foo:example.org",
		Type:     "m.room.member",
		Sender:   "@bob:example.org",
		StateKey: internal.Pointer("@alice:example.org"),
		Content: map[string]any{
			"membership": "doesn't matter",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId:  "$neutral1",
		RoomId:   "!foo:example.org",
		Type:     "m.room.member",
		Sender:   "@alice:example.org",
		StateKey: internal.Pointer("@alice:example.org"),
		Content: map[string]any{
			"membership": "doesn't matter",
		},
	})

	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.OtherGeneral))
	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
}
