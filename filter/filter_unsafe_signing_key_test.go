package filter

import (
	"context"
	"crypto/ed25519"
	"math/rand"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/test"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/stretchr/testify/assert"
)

func TestUnsafeSigningKeyFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			UnsafeSigningKeyFilterEnabled: true,
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{UnsafeSigningKeyFilterName},
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

	_, unsafeKey, err := ed25519.GenerateKey(rand.New(rand.NewSource(0)))
	assert.NoError(t, err)

	_, realKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	}).Sign("example.org", "ed25519:testing", unsafeKey)
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	}).Sign("example.org", "ed25519:testing", realKey)

	assertSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		vecs, err := set.CheckEvent(context.Background(), event, nil)
		assert.NoError(t, err)
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
			assert.Equal(t, 1.0, vecs.GetVector(classification.Unsafe))
		} else {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
			assert.Equal(t, 0.0, vecs.GetVector(classification.Unsafe))
		}
	}
	assertSpamVector(spammyEvent1, true)
	assertSpamVector(neutralEvent1, false)
}
