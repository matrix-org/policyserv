package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestHttpCheckTextCommunityApiWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)
	serverCommunity := createCommunityWithAccessToken(t, api)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut /* should be POST */, "/_policyserv/v1/check/text", bytes.NewBufferString("doesn't matter"))
	httpCheckTextCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	test.AssertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}

func TestHttpCheckTextCommunityApi(t *testing.T) {
	t.Parallel()

	api := makeApi(t)
	serverCommunity := createCommunityWithAccessToken(t, api)

	// Configure a simple keyword filter for the community
	serverCommunity.Config.KeywordFilterKeywords = &[]string{"keyword1", "keyword2"}
	err := api.storage.UpsertCommunity(context.Background(), serverCommunity)
	assert.NoError(t, err)

	// First test a keyword match
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_policyserv/v1/check/text", bytes.NewBufferString("this text contains keyword1 which should flag it"))
	httpCheckTextCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	test.AssertApiError(t, w, "ORG.MATRIX.MSC4387_SAFETY", "Text is probably spammy")

	// Then test a non-match
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/_policyserv/v1/check/text", bytes.NewBufferString("this text doesn't contain any keywords"))
	httpCheckTextCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
