package filter

import (
	"crypto/ed25519"
	"math/rand"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestUnsafeSigningKeyFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			UnsafeSigningKeyFilterEnabled: true,
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{UnsafeSigningKeyFilterName},
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

	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.OtherGeneral))
	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
}
