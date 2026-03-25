package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/sjson"
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

func TestHttpCheckEventIdCommunityApiWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)
	serverCommunity := createCommunityWithAccessToken(t, api)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut /* should be POST */, "/_policyserv/v1/check/event_id", bytes.NewBufferString("doesn't matter"))
	httpCheckEventIdCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	test.AssertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}

func TestHttpCheckEventIdCommunityApiWithKnownEvent(t *testing.T) {
	t.Parallel()

	api := makeApi(t)
	serverCommunity := createCommunityWithAccessToken(t, api)

	// For this test, cache a couple event results to ensure we can avoid federation calls
	err := api.storage.UpsertEventResult(context.Background(), &storage.StoredEventResult{
		EventId:           "$spam",
		IsProbablySpam:    true,
		ConfidenceVectors: confidence.Vectors{classification.Spam: 1.0},
	})
	assert.NoError(t, err)
	err = api.storage.UpsertEventResult(context.Background(), &storage.StoredEventResult{
		EventId:           "$neutral",
		IsProbablySpam:    false,
		ConfidenceVectors: confidence.NewConfidenceVectors(),
	})
	assert.NoError(t, err)

	// Test each case
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_policyserv/v1/check/event_id", bytes.NewBufferString(`{"event_id": "$spam"}`))
	httpCheckEventIdCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	test.AssertApiError(t, w, "M_FORBIDDEN", "This message is not allowed by the policy server")

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/_policyserv/v1/check/event_id", bytes.NewBufferString(`{"event_id": "$neutral"}`))
	httpCheckEventIdCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHttpCheckEventIdCommunityApiWithUnknownEvent(t *testing.T) {
	t.Parallel()

	api := makeApi(t)
	serverCommunity := createCommunityWithAccessToken(t, api)
	roomId := "!room:example.org"

	// Store the room right away
	err := api.storage.UpsertRoom(context.Background(), &storage.StoredRoom{
		RoomId:      roomId,
		RoomVersion: "10",
		CommunityId: serverCommunity.CommunityId,
	})
	assert.NoError(t, err)

	// Set up a keyword filter to flag spammy test events as spam
	serverCommunity.Config.KeywordFilterKeywords = &[]string{"spammy spam"}
	serverCommunity.Config.HellbanPostfilterMinutes = internal.Pointer(-1) // disable hellban filter
	err = api.storage.UpsertCommunity(context.Background(), serverCommunity)
	assert.NoError(t, err)

	// We don't cache events here, and we expect to make outbound federation calls to go get them instead. Set up a mock
	// homeserver and httptest "remote" server for that purpose.
	hs := test.NewMockServer(t, api.storage, func(c *homeserver.Config) {
		c.SkipVerify = true // our httptest server will have an unknown authority
	})
	api.hs = hs
	localhost := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pdu gomatrixserverlib.PDU
		if strings.Contains(r.URL.Path, "$spam") {
			pdu = test.MakeSignedPDU(t, hs, &test.BaseClientEvent{
				RoomId:  roomId,
				Type:    "m.room.message",
				Sender:  "@alice:example.org",
				Content: map[string]any{"body": "spammy spam"},
			})
		} else {
			pdu = test.MakeSignedPDU(t, hs, &test.BaseClientEvent{
				RoomId:  roomId,
				Type:    "m.room.message",
				Sender:  "@alice:example.org",
				Content: map[string]any{"body": "this is a neutral event"},
			})
		}

		// Per spec, the get event endpoint is a single PDU transaction
		w.Header().Set("Content-Type", "application/json")
		txn, err := sjson.SetRawBytes([]byte(`{"pdus":[]}`), "pdus.0", pdu.JSON())
		assert.NoError(t, err) // "should never happen"
		_, _ = w.Write(txn)
	}))
	defer localhost.Close()
	parsed, err := url.Parse(localhost.URL)
	assert.NoError(t, err) // "should never happen"
	api.eventFetchServers = []string{fmt.Sprintf("127.0.0.1:%s", parsed.Port())}

	// Test each case
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_policyserv/v1/check/event_id", bytes.NewBufferString(`{"event_id": "$spam"}`))
	httpCheckEventIdCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	test.AssertApiError(t, w, "M_FORBIDDEN", "This message is not allowed by the policy server")

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/_policyserv/v1/check/event_id", bytes.NewBufferString(`{"event_id": "$neutral"}`))
	httpCheckEventIdCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
