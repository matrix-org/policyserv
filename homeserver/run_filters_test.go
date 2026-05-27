package homeserver

import (
	"context"
	"testing"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/queue"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestRunFiltersQueuesStateLearning(t *testing.T) {
	t.Parallel()

	// Set up a homeserver which activates the happy path of queueLearnStateIfNeeded
	hs := NewMockServerForTest(t, test.NewMemoryStorage(t), func(c *Config) {
		c.TrustedOrigins = []string{"example.org"}
		c.CacheRoomStateFor = 1 * time.Minute
	})
	event := MakeSignedPDUForTest(t, hs, &test.BaseClientEvent{
		RoomId:   "!test:example.org",
		Type:     "m.room.create",
		StateKey: internal.Pointer(""),
		Sender:   "@alice:example.org",
		Content: map[string]any{
			"room_version": "10",
		},
	})
	hs.stateLearner = &expectCreateEventLearner{
		t:                 t,
		inRoomId:          event.RoomID().String(),
		canLearnCallCount: 0,
		doLearnCallCount:  0,
	}
	room := &storage.StoredRoom{
		RoomId:                         event.RoomID().String(),
		RoomVersion:                    "10",
		LastCachedStateTimestampMillis: time.Now().Add(-10 * time.Minute).UnixMilli(),
		CommunityId:                    "default",
	}
	err := hs.storage.UpsertRoom(context.Background(), room)
	assert.NoError(t, err)
	err = hs.storage.UpsertCommunity(context.Background(), &storage.StoredCommunity{
		CommunityId: "default",
		Name:        "Default Testing Community",
		Config: &config.CommunityConfig{
			EventTypePrefilterAllowedStateEventTypes: internal.Pointer([]string{event.Type()}),
			EventTypePrefilterAllowedEventTypes:      internal.Pointer([]string{event.Type()}),
		},
	})
	assert.NoError(t, err)

	// Now we can call the RunFilters code. Note that this might cause side effects because it
	// does a real scan of the event. We're only looking for the particular (async) side effect
	// of state learning happening, though.
	waitCh := make(chan *queue.PoolResult)
	defer close(waitCh)
	err = hs.RunFilters(context.Background(), event, waitCh)
	assert.NoError(t, err)
	// wait for the event to finish processing
	assert.NotNil(t, <-waitCh)
	// wait for the state learning to settle
	time.Sleep(250 * time.Millisecond)
	next, txn, err := hs.storage.PopStateLearnQueue(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, txn)
	assert.Equal(t, event.RoomID().String(), next.RoomId)
	assert.NoError(t, txn.Commit()) // drain the queue ahead of the next test
}
