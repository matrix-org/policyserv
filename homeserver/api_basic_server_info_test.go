package homeserver

import (
	"crypto/ed25519"
	"encoding/base64"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestHttpKeyDiscovery(t *testing.T) {
	t.Parallel()

	// Because we're interacting with very little of the interface, we can create a Homeserver instance without using
	// the NewHomeserver() function.
	_, eventSigningKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)
	server := &Homeserver{
		eventSigningKey: eventSigningKey,
	}

	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		log.Println("Testing method", method)

		res := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/.well-known/matrix/org.matrix.msc4284.policy_server", nil)

		httpKeyDiscovery(server, res, req)
		assert.Equal(t, http.StatusOK, res.Code)
		b64 := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(eventSigningKey.Public().(ed25519.PublicKey))
		test.AssertJsonBody(t, res, map[string]any{"public_key": b64})
		assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
		assert.Equal(t, "*", res.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", res.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "X-Requested-With, Content-Type, Authorization", res.Header().Get("Access-Control-Allow-Headers"))
	}

	// Test wrong method too while we're here
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/.well-known/matrix/org.matrix.msc4284.policy_server", nil)
	httpKeyDiscovery(server, res, req)
	assert.Equal(t, http.StatusMethodNotAllowed, res.Code)
	test.AssertApiError(t, res, "M_UNRECOGNIZED", "Method not allowed")
}
