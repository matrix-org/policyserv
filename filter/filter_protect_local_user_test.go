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
			EnabledNames:           []string{ProtectLocalUserFilterName},
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
	assertSpamVector(neutralEvent1, false)
}
