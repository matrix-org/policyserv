package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/storage"
	"github.com/stretchr/testify/assert"
)

func TestCreateCommunityWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet /*this should be POST*/, "/api/v1/communities/new", makeJsonBody(t, map[string]any{
		"name": "community name",
	}))
	httpCreateCommunityApi(api, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}

func TestCreateCommunityBadJSON(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	notJsonBody := bytes.NewReader(make([]byte, 12))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/communities/new", notJsonBody)
	httpCreateCommunityApi(api, w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assertApiError(t, w, "M_BAD_JSON", "Error")

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/v1/communities/new", makeJsonBody(t, map[string]any{
		"not_name": "name should be required",
	}))
	httpCreateCommunityApi(api, w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assertApiError(t, w, "M_BAD_JSON", "Name is required")

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/v1/communities/new", makeJsonBody(t, map[string]any{
		"name": "               ", // but empty, so should be "missing"
	}))
	httpCreateCommunityApi(api, w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assertApiError(t, w, "M_BAD_JSON", "Name is required")
}

func TestCreateCommunityCreate(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	communityName := "Test Community"
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/communities/new", makeJsonBody(t, map[string]any{
		"name": communityName,
	}))
	httpCreateCommunityApi(api, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	community := &storage.StoredCommunity{}
	err := json.Unmarshal(w.Body.Bytes(), community)
	assert.NoError(t, err)
	assert.Equal(t, communityName, community.Name)
	assert.NotEmpty(t, community.CommunityId)
	assert.NotNil(t, community.Config)

	// Ensure it was also stored
	fromDb, err := api.storage.GetCommunity(context.Background(), community.CommunityId)
	assert.NoError(t, err)
	assert.NotNil(t, fromDb)
	assert.Equal(t, communityName, fromDb.Name)
	assert.Equal(t, community.CommunityId, fromDb.CommunityId)

	// Note: we can't (currently) test that errors during database calls and HTTP responses are handled. A future test
	// case *should* cover this.
}

func TestGetCommunityWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost /*this should be GET*/, "/api/v1/communities/not_a_real_id", nil)
	httpGetCommunityApi(api, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}

func TestGetCommunityNotFound(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/communities/not_a_real_id", nil)
	r.SetPathValue("id", "not_a_real_id")
	httpGetCommunityApi(api, w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assertApiError(t, w, "M_NOT_FOUND", "Community not found")
}

func TestGetCommunity(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	name := "Test Community"
	community, err := api.storage.CreateCommunity(context.Background(), name)
	assert.NoError(t, err)
	assert.NotNil(t, community)
	assert.NotEmpty(t, community.CommunityId)
	assert.Equal(t, name, community.Name)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/communities/"+community.CommunityId, nil)
	r.SetPathValue("id", community.CommunityId)
	httpGetCommunityApi(api, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	fromRes := &storage.StoredCommunity{}
	err = json.Unmarshal(w.Body.Bytes(), fromRes)
	assert.NoError(t, err)
	assert.Equal(t, community, fromRes)

	// Note: we can't (currently) test that errors during database calls and HTTP responses are handled. A future test
	// case *should* cover this.
}

func TestSetCommunityConfigWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet /*this should be POST*/, "/api/v1/communities/not_a_real_id/config", nil)
	httpSetCommunityConfigApi(api, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}

func TestSetCommunityConfigNotFound(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	cnf := &config.CommunityConfig{
		KeywordFilterKeywords: []string{"keyword1", "keyword2"},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/communities/not_a_real_id/config", makeJsonBody(t, cnf))
	r.SetPathValue("id", "not_a_real_id")
	httpSetCommunityConfigApi(api, w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assertApiError(t, w, "M_NOT_FOUND", "Community not found")
}

func TestSetCommunityConfig(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	name := "Test Community"
	community, err := api.storage.CreateCommunity(context.Background(), name)
	assert.NoError(t, err)
	assert.NotNil(t, community)
	assert.NotEmpty(t, community.CommunityId)
	assert.Equal(t, name, community.Name)

	cnf := &config.CommunityConfig{
		KeywordFilterKeywords: []string{"keyword1", "keyword2"},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/communities/"+community.CommunityId+"/config", makeJsonBody(t, cnf))
	r.SetPathValue("id", community.CommunityId)
	httpSetCommunityConfigApi(api, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	fromRes := &storage.StoredCommunity{}
	err = json.Unmarshal(w.Body.Bytes(), fromRes)
	assert.NoError(t, err)
	assert.NotEqual(t, community, fromRes) // the config should have changed...
	community.Config = cnf                 // ...to this
	assert.Equal(t, community, fromRes)

	// Note: we can't (currently) test that errors during database calls and HTTP responses are handled. A future test
	// case *should* cover this.
}
