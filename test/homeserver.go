package test

import (
	"context"
	"crypto/ed25519"
	"sync"
	"testing"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/queue"
	"github.com/stretchr/testify/assert"
)

var generatedSigningKeys = sync.Map{}

var NoConfigChanges func(c *homeserver.Config) = nil

func NewMockServer(t *testing.T, configModFn func(c *homeserver.Config)) *homeserver.Homeserver {
	_, eventSigningKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)
	cnf := &homeserver.Config{
		// We only set the values we *need* to. Otherwise we expect that the created Homeserver will be fine without
		// all the extra config
		ServerName:             "policy.example.org",
		PrivateSigningKey:      eventSigningKey,
		PrivateEventSigningKey: eventSigningKey,
		SigningKeyVersion:      "1",
		ActorLocalpart:         "policyserv",
		CacheRoomStateFor:      24 * time.Hour,
		KeyQueryServer: &homeserver.KeyQueryServer{
			Name:           "noop.example.org",
			PreferredKeyId: "ed25519:invalid",
			PreferredKey:   nil,
		},
	}
	if configModFn != nil {
		configModFn(cnf)
	}
	instanceCnf, err := config.NewInstanceConfig()
	assert.NoError(t, err)
	assert.NotNil(t, instanceCnf)
	storage := NewMemoryStorage(t)
	pubsub := NewMemoryPubsub(t)
	communityManager, err := community.NewManager(instanceCnf, storage, pubsub, MustMakeAuditQueue(5))
	assert.NoError(t, err)
	assert.NotNil(t, communityManager)
	pool, err := queue.NewPool(&queue.PoolConfig{
		ConcurrentPools: 5,
		SizePerPool:     10,
	}, communityManager, storage)
	assert.NoError(t, err)
	assert.NotNil(t, pool)

	server, err := homeserver.NewHomeserver(cnf, storage, pool, pubsub)
	assert.NoError(t, err)
	assert.NotNil(t, server)

	return server
}

func CreateAndInjectOrigin(t *testing.T, hs *homeserver.Homeserver, originName string) (gomatrixserverlib.KeyID, ed25519.PrivateKey) {
	originKeyId := gomatrixserverlib.KeyID("ed25519:1")
	originPublicKey, originPrivateKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	key, _ := generatedSigningKeys.LoadOrStore(originName, originPrivateKey)
	originPrivateKey = key.(ed25519.PrivateKey)
	originPublicKey = originPrivateKey.Public().(ed25519.PublicKey)

	// Store the key in the server's keyring too
	err = hs.StoreKeys(context.Background(), map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult{
		gomatrixserverlib.PublicKeyLookupRequest{
			ServerName: spec.ServerName(originName),
			KeyID:      originKeyId,
		}: {
			ExpiredTS:    gomatrixserverlib.PublicKeyNotExpired,
			ValidUntilTS: spec.AsTimestamp(time.Now().Add(24 * time.Hour)),
			VerifyKey: gomatrixserverlib.VerifyKey{
				Key: spec.Base64Bytes(originPublicKey),
			},
		},
	})
	assert.NoError(t, err)

	return originKeyId, originPrivateKey
}
