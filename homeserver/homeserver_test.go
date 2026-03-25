package homeserver

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
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

var generatedSigningKeys = sync.Map{}

var NoConfigChanges func(c *Config) = nil

func NewMockServer(t *testing.T, configModFn func(c *Config)) *Homeserver {
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
	if configModFn != nil {
		configModFn(cnf)
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
	originPublicKey, originPrivateKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)

	key, _ := generatedSigningKeys.LoadOrStore(originName, originPrivateKey)
	originPrivateKey = key.(ed25519.PrivateKey)
	originPublicKey = originPrivateKey.Public().(ed25519.PublicKey)

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

func TestAllowedDeniedNetworks(t *testing.T) {
	t.Parallel()

	hs := NewMockServer(t, func(c *Config) {
		c.AllowedNetworks = []string{"127.0.0.1/32"}
		c.DeniedNetworks = []string{"127.0.0.2/32"}
		c.SkipVerify = true // our httptest server will have an unknown authority
	})

	// Set up a test server that listens on localhost (127.0.0.0/8)
	// We'll use this for the "allowed networks" check. We need to use a TLS Server because fclient from
	// GMSL will *always* connect over HTTPS.
	responseCount := 0
	localhost := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key": "val"}`))
	}))
	defer localhost.Close()
	parsed, err := url.Parse(localhost.URL)
	assert.NoError(t, err) // "should never happen"
	localhostPort := parsed.Port()

	// Try to connect to 127.0.0.1 (an allowed network)
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+localhostPort, nil)
	assert.NoError(t, err) // "should never happen"
	res := make(map[string]string)
	err = hs.client.DoRequestAndParseResponse(context.Background(), req, &res)
	assert.NoError(t, err)
	assert.Equal(t, "val", res["key"])
	assert.Equal(t, 1, responseCount)

	// Try to connect to 127.0.0.2 (a denied network)
	req, err = http.NewRequest(http.MethodGet, "http://127.0.0.2:"+localhostPort, nil)
	assert.NoError(t, err) // "should never happen"
	err = hs.client.DoRequestAndParseResponse(context.Background(), req, &res)
	assert.Error(t, err)
	// Example (port number is variable): Get "http://127.0.0.2:63780": dial tcp 127.0.0.2:63780: 127.0.0.2:63780 is denied
	assert.ErrorContains(t, err, "dial tcp 127.0.0.2:")
	assert.ErrorContains(t, err, " is denied")
	assert.Equal(t, 1, responseCount) // we should have never connected

	// Try to connect to 127.0.0.3 (an implicitly denied network)
	req, err = http.NewRequest(http.MethodGet, "http://127.0.0.3:"+localhostPort, nil)
	assert.NoError(t, err) // "should never happen"
	err = hs.client.DoRequestAndParseResponse(context.Background(), req, &res)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "dial tcp 127.0.0.3:") // same error as above, hopefully
	assert.ErrorContains(t, err, " is denied")
	assert.Equal(t, 1, responseCount) // we should have never connected
}
