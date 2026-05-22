package homeserver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"testing/synctest"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/sjson"
)

func TestShouldLearnState(t *testing.T) {
	t.Parallel()

	// We use synctest to manipulate time itself - see https://go.dev/blog/testing-time#time
	synctest.Test(t, func(t *testing.T) {
		// Dev note: We'd ideally use NewMockServerForTest here, but it ends up creating a bunch of stuff we
		// don't need, like worker pools for filtering events. These other dependencies spin up a lot of
		// goroutines which we can't easily shut down, which prevents synctest from passing. So, we create
		// our own minimal homeserver instance with exactly what we need.
		hs := &Homeserver{
			storage:           test.NewMemoryStorage(t),
			cacheRoomStateFor: 1 * time.Minute,
		}

		// Advance time by a little bit so we can do subtraction without overflow issues
		time.Sleep(1 * time.Hour)

		testCases := []struct {
			lastCachedTimestamp int64
			expectedShouldLearn bool
		}{
			{
				// The room's last cached state was older than the CacheRoomStateFor time - shouldLearnState should be true
				lastCachedTimestamp: time.Now().Add(-2 * time.Minute).UnixMilli(),
				expectedShouldLearn: true,
			},
			{
				// The room's last cached state is in the future (indicating it hasn't been expired) - shouldLearnState should be false
				lastCachedTimestamp: time.Now().Add(2 * time.Minute).UnixMilli(),
				expectedShouldLearn: false,
			},
			{
				// When the room's last cached timestamp is exactly equal to CacheRoomStateFor's duration, we should learn state
				lastCachedTimestamp: time.Now().Add(-1 * time.Minute).UnixMilli(),
				expectedShouldLearn: true,
			},
			{
				// When the cache is even 1ms too new, we should not learn state
				lastCachedTimestamp: time.Now().Add(-1*time.Minute).UnixMilli() + 1,
				expectedShouldLearn: false,
			},
		}

		for i, tc := range testCases {
			t.Logf("Test case %d: lastCachedTimestamp=%d, expectedShouldLearn=%t", i, tc.lastCachedTimestamp, tc.expectedShouldLearn)

			// Insert the room to test it
			room := &storage.StoredRoom{
				RoomId:                         fmt.Sprintf("!%s:example.org", storage.NextId()),
				LastCachedStateTimestampMillis: tc.lastCachedTimestamp,
				RoomVersion:                    "10",
			}
			err := hs.storage.UpsertRoom(context.Background(), room)
			assert.NoError(t, err)

			shouldLearn, room2, err := hs.shouldLearnState(context.Background(), room.RoomId)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedShouldLearn, shouldLearn)
			assert.Equal(t, room.RoomId, room2.RoomId)
		}

		synctest.Wait() // wait for goroutines to finish
	})
}

func TestLearnState(t *testing.T) {
	t.Parallel()

	hs := NewMockServerForTest(t, test.NewMemoryStorage(t), func(c *Config) {
		c.SkipVerify = true // our httptest server will have an unknown authority
	})

	// We should get an error on unknown room IDs
	assert.EqualError(t, hs.LearnState(context.Background(), "!unknown:example.org", "$event", "example.org"), "room !unknown:example.org not found")

	// Insert a room we'll use to test
	room := &storage.StoredRoom{
		RoomId:      "!test:example.org",
		RoomVersion: "10",
		// We set the last cached timestamp to be far in the future to ensure we *never* rely on this while learning
		// state. When we ask to learn state, we want to always do that even if we recently learned state.
		LastCachedStateTimestampMillis: time.Now().Add(500 * time.Hour).UnixMilli(),
	}
	err := hs.storage.UpsertRoom(context.Background(), room)
	assert.NoError(t, err)

	// Set up a server to learn state from
	expectedAtEventId := "$at_this_event"
	handlerCalled := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		assert.Equal(t, "/_matrix/federation/v1/state/"+room.RoomId, r.URL.Path)
		assert.Equal(t, fmt.Sprintf("event_id=%s", url.QueryEscape(expectedAtEventId)), r.URL.RawQuery)

		createEvent := MakeSignedPDUForTest(t, hs, &test.BaseClientEvent{
			RoomId:   room.RoomId,
			Type:     "m.room.create",
			StateKey: internal.Pointer(""),
			Sender:   "@alice:example.org",
			Content: map[string]any{
				"room_version": room.RoomVersion,
			},
		})

		// Both the auth_chain and pdus are equivalent in this test, so populate both with the create event
		resp, err := sjson.SetRawBytes([]byte(`{"auth_chain": [], "pdus": []}`), "auth_chain.0", createEvent.JSON())
		assert.NoError(t, err)
		resp, err = sjson.SetRawBytes(resp, "pdus.0", createEvent.JSON())
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

	// Override the state learner so we can detect that it was called properly
	hs.stateLearner = &expectCreateEventLearner{
		t:                 t,
		inRoomId:          room.RoomId,
		canLearnCallCount: 0,
		doLearnCallCount:  0,
	}

	// Finally, we can make the state learning call
	err = hs.LearnState(context.Background(), room.RoomId, expectedAtEventId, fmt.Sprintf("127.0.0.1:%s", localhostPort))
	assert.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.Equal(t, 0, hs.stateLearner.(*expectCreateEventLearner).canLearnCallCount) // not called in this path
	assert.Equal(t, 1, hs.stateLearner.(*expectCreateEventLearner).doLearnCallCount)
}

type expectCreateEventLearner struct {
	t                 *testing.T
	canLearnCallCount int
	doLearnCallCount  int
	inRoomId          string
}

func (e *expectCreateEventLearner) CanLearn(ctx context.Context, room *storage.StoredRoom, event gomatrixserverlib.PDU) (bool, error) {
	e.canLearnCallCount++
	assert.NotNil(e.t, ctx, "context is required")
	return event.Type() == "m.room.create" && room.RoomId == e.inRoomId, nil
}

func (e *expectCreateEventLearner) LearnFrom(ctx context.Context, room *storage.StoredRoom, roomState []gomatrixserverlib.PDU) error {
	e.doLearnCallCount++
	assert.NotNil(e.t, ctx, "context is required")
	assert.Equal(e.t, e.inRoomId, room.RoomId)
	assert.Equal(e.t, room.RoomId, roomState[0].RoomID().String())
	assert.Equal(e.t, 1, len(roomState))
	assert.Equal(e.t, "m.room.create", roomState[0].Type())
	return nil
}
