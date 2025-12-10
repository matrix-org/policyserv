package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/storage"
	"github.com/stretchr/testify/assert"
)

func TestGetRoomWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost /*this should be GET*/, "/api/v1/rooms/!room:example.org", nil)
	r.SetPathValue("id", "!room:example.org")
	httpGetRoomApi(api, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}

func TestGetRoomNotFound(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/rooms/not_a_real_id", nil)
	r.SetPathValue("id", "not_a_real_id")
	httpGetRoomApi(api, w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assertApiError(t, w, "M_NOT_FOUND", "Room not found")
}

func TestGetRoom(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/rooms/!room:example.org", nil)
	r.SetPathValue("id", "!room:example.org")
	room := &storage.StoredRoom{
		RoomId:                         "!room:example.org",
		RoomVersion:                    "11",
		ModeratorUserId:                "@mod:example.org",
		LastCachedStateTimestampMillis: 1234,
		CommunityId:                    "default",
	}
	err := api.storage.UpsertRoom(ctx, room)
	assert.NoError(t, err)
	httpGetRoomApi(api, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	fromRes := &storage.StoredRoom{}
	err = json.Unmarshal(w.Body.Bytes(), fromRes)
	assert.NoError(t, err)
	assert.Equal(t, room, fromRes)
}

func TestAddRoomWrongMethod(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet /*this should be POST*/, "/api/v1/rooms/!room:example.org/join", nil)
	r.SetPathValue("roomId", "!room:example.org")
	httpAddRoomApi(api, w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assertApiError(t, w, "M_UNRECOGNIZED", "Method not allowed")
}

func TestAddRoomUnknownCommunity(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/!room:example.org/join", makeJsonBody(t, map[string]any{
		"community_id": "does_not_exist",
	}))
	r.SetPathValue("roomId", "!room:example.org")
	httpAddRoomApi(api, w, r)
	assertApiError(t, w, "M_BAD_STATE", "Community not found")
}

func TestAddRoomUnknownCommunityWhenNoneSupplied(t *testing.T) {
	t.Parallel()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/!room:example.org/join", makeJsonBody(t, map[string]any{
		//"community_id": "ID goes here", // we're testing what happens when we don't supply this, so don't.
	}))
	r.SetPathValue("roomId", "!room:example.org")
	httpAddRoomApi(api, w, r)
	assertApiError(t, w, "M_BAD_STATE", "Community not found")
}

func TestAddRoomAlreadyJoined(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	api := makeApi(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/!room:example.org/join", makeJsonBody(t, map[string]any{
		"community_id": "default",
	}))
	r.SetPathValue("roomId", "!room:example.org")
	community := &storage.StoredCommunity{
		CommunityId: "default",
		Name:        "default",
		Config:      &config.CommunityConfig{},
	}
	err := api.storage.UpsertCommunity(ctx, community)
	assert.NoError(t, err)
	room := &storage.StoredRoom{
		RoomId:                         "!room:example.org",
		RoomVersion:                    "11",
		ModeratorUserId:                "@mod:example.org",
		LastCachedStateTimestampMillis: 1234,
		CommunityId:                    "default",
	}
	err = api.storage.UpsertRoom(ctx, room)
	assert.NoError(t, err)
	httpAddRoomApi(api, w, r)
	assertApiError(t, w, "M_BAD_STATE", "Room already exists")
}

func TestAddRoom(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	api := makeApi(t)

	// TODO: Mock the homeserver to prevent JoinRoom from using real code
	t.Skip("skip because the homeserver cannot be mocked at the moment. See https://github.com/matrix-org/policyserv/issues/20")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/!room:example.org/join", makeJsonBody(t, map[string]any{
		"community_id": "non_default",
	}))
	r.SetPathValue("roomId", "!room:example.org")
	community := &storage.StoredCommunity{
		CommunityId: "non_default",
		Name:        "non_default",
		Config:      &config.CommunityConfig{},
	}
	err := api.storage.UpsertCommunity(ctx, community)
	assert.NoError(t, err)
	httpAddRoomApi(api, w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	fromRes := &storage.StoredRoom{}
	err = json.Unmarshal(w.Body.Bytes(), fromRes)
	assert.NoError(t, err)
	assert.Equal(t, &storage.StoredRoom{
		RoomId:                         "!room:example.org",
		RoomVersion:                    "11", // dependent on JoinRoom implementation
		ModeratorUserId:                "",
		LastCachedStateTimestampMillis: 0,
		CommunityId:                    "non_default",
	}, fromRes)
}
