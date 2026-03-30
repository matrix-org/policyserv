package homeserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/fclient"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func (h *Homeserver) MustMakeFederationRequest(t *testing.T, method string, uriPath string, content interface{}, originName string) *http.Request {
	originKeyId, originPrivateKey := CreateAndInjectOriginForTest(t, h, originName)
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

func TestAllowedDeniedNetworks(t *testing.T) {
	t.Parallel()

	hs := NewMockServerForTest(t, test.NewMemoryStorage(t), func(c *Config) {
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
