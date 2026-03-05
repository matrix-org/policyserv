package homeserver

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"testing"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/fclient"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

var generatedSigningKeys = make(map[string]ed25519.PrivateKey)

func NewMockServer(t *testing.T) *Homeserver {
	_, eventSigningKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)
	cnf := &Config{
		// We only set the values we *need* to. Otherwise we expect that the created Homeserver will be fine without
		// all the extra config
		ServerName:             "policy.example.org",
		PrivateSigningKey:      eventSigningKey,
		PrivateEventSigningKey: eventSigningKey,
		SigningKeyVersion:      "1",
		ActorLocalpart:         "policyserv",
		CacheRoomStateFor:      24 * time.Hour,
		KeyQueryServer: &KeyQueryServer{
			Name:           "noop.example.org",
			PreferredKeyId: "ed25519:invalid",
			PreferredKey:   nil,
		},
	}
	instanceCnf, err := config.NewInstanceConfig()
	assert.NoError(t, err)
	assert.NotNil(t, instanceCnf)
	storage := test.NewMemoryStorage(t)
	pubsub := test.NewMemoryPubsub(t)
	communityManager, err := community.NewManager(instanceCnf, storage, pubsub, test.MustMakeAuditQueue(5))
	assert.NoError(t, err)
	assert.NotNil(t, communityManager)
	pool, err := queue.NewPool(&queue.PoolConfig{
		ConcurrentPools: 5,
		SizePerPool:     10,
	}, communityManager, storage)
	assert.NoError(t, err)
	assert.NotNil(t, pool)

	server, err := NewHomeserver(cnf, storage, pool, pubsub)
	assert.NoError(t, err)
	assert.NotNil(t, server)

	return server
}

func (h *Homeserver) MustMakeFederationRequest(t *testing.T, method string, uriPath string, content interface{}, originName string) *http.Request {
	originKeyId, originPrivateKey := newOriginSigningKey(t, h, originName)
	if event, ok := content.(gomatrixserverlib.PDU); ok {
		// Sign events in case the test doesn't
		content = event.Sign(originName, originKeyId, originPrivateKey)
	}

	fedReq := fclient.NewFederationRequest(method, spec.ServerName(originName), h.ServerName, uriPath)
	err := fedReq.SetContent(content)
	assert.NoError(t, err)
	err = fedReq.Sign(spec.ServerName(originName), originKeyId, originPrivateKey)
	assert.NoError(t, err)
	req, err := fedReq.HTTPRequest()
	assert.NoError(t, err)
	return req
}

func newOriginSigningKey(t *testing.T, server *Homeserver, originName string) (gomatrixserverlib.KeyID, ed25519.PrivateKey) {
	originKeyId := gomatrixserverlib.KeyID("ed25519:1")
	if key, ok := generatedSigningKeys[originName]; ok {
		return originKeyId, key
	}
	originPublicKey, originPrivateKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)
	generatedSigningKeys[originName] = originPrivateKey

	// Store the key in the server's keyring too
	err = server.StoreKeys(context.Background(), map[gomatrixserverlib.PublicKeyLookupRequest]gomatrixserverlib.PublicKeyLookupResult{
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
