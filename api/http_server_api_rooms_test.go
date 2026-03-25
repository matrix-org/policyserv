package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func setCommunitySelfJoinRoomsPermission(t *testing.T, api *Api, serverCommunity *storage.StoredCommunity, allowed bool) {
	serverCommunity.CanSelfJoinRooms = allowed
	err := api.storage.UpsertCommunity(context.Background(), serverCommunity)
	assert.NoError(t, err)
}

func TestJoinRoomCommunityApiWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)
	serverCommunity := createCommunityWithAccessToken(t, api)
	setCommunitySelfJoinRoomsPermission(t, api, serverCommunity, true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut /* should be POST */, "/_policyserv/v1/join/!room:example.org", bytes.NewBufferString("doesn't matter"))
	r.SetPathValue("roomId", "!room:example.org")
	httpJoinRoomCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	test.AssertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}

func TestJoinRoomCommunityApiAlreadyJoined(t *testing.T) {
	t.Parallel()

	api := makeApi(t)
	serverCommunity := createCommunityWithAccessToken(t, api)
	setCommunitySelfJoinRoomsPermission(t, api, serverCommunity, true)

	err := api.storage.UpsertRoom(context.Background(), &storage.StoredRoom{
		RoomId:                         "!room:example.org",
		RoomVersion:                    "11",
		ModeratorUserId:                "@moderator:example.org",
		LastCachedStateTimestampMillis: 1234,
		CommunityId:                    serverCommunity.CommunityId,
	})
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_policyserv/v1/join/!room:example.org", bytes.NewBufferString("doesn't matter"))
	r.SetPathValue("roomId", "!room:example.org")
	httpJoinRoomCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	test.AssertApiError(t, w, "M_BAD_STATE", "Room already exists")
}

func TestJoinRoomCommunityApiNotAllowedToJoin(t *testing.T) {
	t.Parallel()

	api := makeApi(t)
	serverCommunity := createCommunityWithAccessToken(t, api)
	// first we test the defaults, which should be false. Then we'll explicitly set the permission to false.

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_policyserv/v1/join/!room:example.org", bytes.NewBufferString("doesn't matter"))
	r.SetPathValue("roomId", "!room:example.org")
	httpJoinRoomCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
	test.AssertApiError(t, w, "M_FORBIDDEN", "This community cannot self-serve add rooms")

	// Test again, with explicit false on the permission
	setCommunitySelfJoinRoomsPermission(t, api, serverCommunity, false)
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/_policyserv/v1/join/!room:example.org", bytes.NewBufferString("doesn't matter"))
	r.SetPathValue("roomId", "!room:example.org")
	httpJoinRoomCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusForbidden, w.Code)
	test.AssertApiError(t, w, "M_FORBIDDEN", "This community cannot self-serve add rooms")
}

func TestJoinRoomCommunityApi(t *testing.T) {
	t.Parallel()

	// TODO: Mock the homeserver to prevent JoinRoom from using real code
	t.Skip("skip because the homeserver cannot be mocked at the moment. See https://github.com/matrix-org/policyserv/issues/20")

	api := makeApi(t)
	serverCommunity := createCommunityWithAccessToken(t, api)
	setCommunitySelfJoinRoomsPermission(t, api, serverCommunity, true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_policyserv/v1/join/!room:example.org", bytes.NewBufferString("doesn't matter"))
	r.SetPathValue("roomId", "!room:example.org")
	httpJoinRoomCommunityApi(api, serverCommunity, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	fromRes := &storage.StoredRoom{}
	err := json.Unmarshal(w.Body.Bytes(), fromRes)
	assert.NoError(t, err)
	assert.Equal(t, &storage.StoredRoom{
		RoomId:                         "!room:example.org",
		RoomVersion:                    "11", // dependent on JoinRoom implementation
		ModeratorUserId:                "",
		LastCachedStateTimestampMillis: 0,
		CommunityId:                    serverCommunity.CommunityId,
	}, fromRes)
}
