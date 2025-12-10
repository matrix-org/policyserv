package queue

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/community"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestPool(t *testing.T) {
	cnf, err := config.NewInstanceConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cnf)

	db := test.NewMemoryStorage(t)
	defer db.Close()

	pubsub := test.NewMemoryPubsub(t)
	defer pubsub.Close()

	manager, err := community.NewManager(cnf, db, pubsub, test.MustMakeAuditQueue(5))
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	pool, err := NewPool(&PoolConfig{
		ConcurrentPools: 1,
		SizePerPool:     5,
	}, manager, db)
	assert.NoError(t, err)
	assert.NotNil(t, pool)

	// Create a test event
	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$event1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@test1:example.org",
		Content: map[string]interface{}{
			"body": "test",
		},
	})

	// Cache a result to ensure the happy path works
	res := &storage.StoredEventResult{
		EventId:           event.EventID(),
		IsProbablySpam:    true,
		ConfidenceVectors: confidence.Vectors{classification.Spam: 0.5, classification.Mentions: 1.0},
	}
	err = db.UpsertEventResult(context.Background(), res)
	assert.NoError(t, err)

	ch := make(chan *PoolResult, 1)
	err = pool.Submit(context.Background(), event, nil, ch)
	assert.NoError(t, err)

	poolResult := <-ch
	assert.NotNil(t, poolResult)
	assert.Equal(t, &PoolResult{
		Vectors:        res.ConfidenceVectors,
		IsProbablySpam: res.IsProbablySpam,
		Err:            nil,
	}, poolResult)
}

func TestPoolHandlesErrors(t *testing.T) {
	cnf, err := config.NewInstanceConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cnf)

	db := test.NewMemoryStorage(t)
	defer db.Close()

	pubsub := test.NewMemoryPubsub(t)
	defer pubsub.Close()

	manager, err := community.NewManager(cnf, db, pubsub, test.MustMakeAuditQueue(5))
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	pool, err := NewPool(&PoolConfig{
		ConcurrentPools: 1,
		SizePerPool:     5,
	}, manager, db)
	assert.NoError(t, err)
	assert.NotNil(t, pool)

	// Create a test event
	event := test.MustMakePDU(&test.BaseClientEvent{
		EventId: test.ErrorEventResultId,
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@test1:example.org",
		Content: map[string]interface{}{
			"body": "should cause an error based on ID",
		},
	})

	// This test doesn't need to persist an event because the memory storage will return an error based on the event ID

	ch := make(chan *PoolResult, 1)
	err = pool.Submit(context.Background(), event, nil, ch)
	assert.NoError(t, err)

	poolResult := <-ch
	assert.NotNil(t, poolResult)
	assert.Equal(t, &PoolResult{
		Vectors:        nil,
		IsProbablySpam: false,
		Err:            test.SimulatedError,
	}, poolResult)
}
