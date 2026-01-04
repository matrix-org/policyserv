package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestGetInstanceConfigWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost /*this should be GET*/, "/api/v1/instance/community_config", nil)
	httpGetInstanceConfigApi(api, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	test.AssertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}

func TestGetInstanceConfig(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	cnf, err := config.NewCommunityConfigForJSON(nil)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/instance/community_config", nil)
	httpGetInstanceConfigApi(api, w, r)
	fromRes := &config.CommunityConfig{}
	err = json.Unmarshal(w.Body.Bytes(), fromRes)
	assert.NoError(t, err)
	assert.Equal(t, cnf, fromRes)
}
