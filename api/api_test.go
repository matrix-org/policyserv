package api

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func makeJsonBody(t *testing.T, body any) io.Reader {
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.NotNil(t, b)
	return bytes.NewReader(b)
}

func assertApiError(t *testing.T, w *httptest.ResponseRecorder, errcode string, error string) {
	jsonErr := make(map[string]any)
	err := json.Unmarshal(w.Body.Bytes(), &jsonErr)
	assert.NoError(t, err)
	assert.Equal(t, errcode, jsonErr["errcode"])
	assert.Equal(t, error, jsonErr["error"])
}

func assertJsonBody(t *testing.T, w *httptest.ResponseRecorder, expected any) {
	expectedJson, err := json.Marshal(expected)
	assert.NoError(t, err)
	assert.JSONEq(t, string(expectedJson), w.Body.String())
}

const testApiKey = "do_not_use_in_production_otherwise_sadness_will_be_created"

func makeApi(t *testing.T) *Api {
	cnf, err := config.NewInstanceConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cnf)

	db := test.NewMemoryStorage(t)
	assert.NotNil(t, db)

	pubsub := test.NewMemoryPubsub(t)
	assert.NotNil(t, pubsub)

	communityManager, err := community.NewManager(cnf, db, pubsub, test.MustMakeAuditQueue(5))
	assert.NoError(t, err)
	assert.NotNil(t, communityManager)

	pool, err := queue.NewPool(&queue.PoolConfig{
		ConcurrentPools: 5,
		SizePerPool:     10,
	}, communityManager, db)
	assert.NoError(t, err)
	assert.NotNil(t, pool)

	hs, err := homeserver.NewHomeserver(&homeserver.Config{
		ServerName: "example.org",
		KeyQueryServer: &homeserver.KeyQueryServer{
			Name:           "example.org",
			PreferredKeyId: "abc",
			PreferredKey:   make([]byte, ed25519.PublicKeySize),
		},
		ActorLocalpart: "admin",
		// we don't need the other fields for these tests
	}, db, pool, pubsub)
	assert.NoError(t, err)
	assert.NotNil(t, hs)

	api, err := NewApi(&Config{
		ApiKey: testApiKey,
	}, db, hs)
	assert.NoError(t, err)
	assert.NotNil(t, api)

	return api
}

func TestAuthenticatedApiNoAuth(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/example", nil)
	//r.Header.Set("Authorization", "Bearer WRONG_TOKEN") // we don't want auth on this test, so don't set it
	upstream := func(a *Api, w http.ResponseWriter, r *http.Request) {
		assert.Fail(t, "should not be called")
	}
	handler := api.httpAuthenticatedRequestHandler(upstream)
	handler.ServeHTTP(w, r)
	assert.Equal(t, w.Code, http.StatusUnauthorized)
	assertApiError(t, w, "M_UNAUTHORIZED", "Not allowed")
}

func TestAuthenticatedApiWrongAuth(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/example", nil)
	r.Header.Set("Authorization", "Bearer WRONG_TOKEN")
	upstream := func(a *Api, w http.ResponseWriter, r *http.Request) {
		assert.Fail(t, "should not be called")
	}
	handler := api.httpAuthenticatedRequestHandler(upstream)
	handler.ServeHTTP(w, r)
	assert.Equal(t, w.Code, http.StatusUnauthorized)
	assertApiError(t, w, "M_UNAUTHORIZED", "Not allowed")
}

func TestAuthenticatedApi(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/example", nil)
	r.Header.Set("Authorization", "Bearer "+api.apiKey)
	called := false
	upstream := func(a *Api, w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	handler := api.httpAuthenticatedRequestHandler(upstream)
	handler.ServeHTTP(w, r)
	assert.Equal(t, w.Code, http.StatusOK)
	assert.True(t, called)
}
