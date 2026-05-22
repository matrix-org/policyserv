package tasks

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/matrix-org/policyserv/homeserver"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/matrix-org/policyserv/trust"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/sjson"
)

func TestCacheLearnedRoomStateTask(t *testing.T) {
	t.Parallel()

	db := test.NewMemoryStorage(t)
	hs := homeserver.NewMockServerForTest(t, db, func(c *homeserver.Config) {
		c.SkipVerify = true // out httptest server will have an unknown authority
	})

	// Prepare a room to test the state queue
	room := &storage.StoredRoom{
		RoomId:      "!test:example.org",
		RoomVersion: "10",
	}
	err := db.UpsertRoom(context.Background(), room)
	assert.NoError(t, err)

	// Prepare a test server to "learn" state from
	handlerCalled := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true

		// We use a power levels event so we can detect that at least one of the learners worked
		powerLevelsEvent := homeserver.MakeSignedPDUForTest(t, hs, &test.BaseClientEvent{
			RoomId:   room.RoomId,
			Type:     "m.room.power_levels",
			StateKey: internal.Pointer(""),
			Sender:   "@alice:example.org",
			Content: map[string]any{
				"users": map[string]any{
					"@alice:example.org": 100,
				},
				"state_default": 50,
				"users_default": 0,
			},
		})

		// Both the auth_chain and pdus are equivalent in this test, so populate both with the create event
		resp, err := sjson.SetRawBytes([]byte(`{"auth_chain": [], "pdus": []}`), "auth_chain.0", powerLevelsEvent.JSON())
		assert.NoError(t, err)
		resp, err = sjson.SetRawBytes(resp, "pdus.0", powerLevelsEvent.JSON())
		assert.NoError(t, err)

		// Respond accordingly
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	}
	localhost := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer localhost.Close()
	parsed, err := url.Parse(localhost.URL)
	assert.NoError(t, err) // "should never happen"
	localhostPort := parsed.Port()

	// Queue the task to learn the room's state
	err = db.PushStateLearnQueue(context.Background(), &storage.StateLearnQueueItem{
		RoomId:               room.RoomId,
		AtEventId:            "$at_this_event",
		ViaServer:            fmt.Sprintf("127.0.0.1:%s", localhostPort),
		AfterTimestampMillis: 0, // effectively "now"
	})
	assert.NoError(t, err)

	// Now we can call the task to ensure that the room's state is "learned"
	CacheLearnedRoomState(hs, db)

	// Verify the state was learned
	assert.Equal(t, true, handlerCalled)
	plSource, err := trust.NewPowerLevelsSource(db)
	assert.NoError(t, err)
	ok, err := plSource.IsUserAboveDefault(context.Background(), room.RoomId, "@alice:example.org")
	assert.NoError(t, err)
	assert.True(t, ok)
}
