package test

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/pubsub"
	"github.com/stretchr/testify/assert"
)

// We want to make sure our test fixture logic is accurate too
func TestMemoryPubsub(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ps := NewMemoryPubsub(t)
	topic := "test"

	// Single subscriber
	ch1, err := ps.Subscribe(ctx, topic)
	assert.NoError(t, err)
	assert.NotNil(t, ch1)

	val := "publish val"
	err = ps.Publish(ctx, topic, val)
	assert.NoError(t, err)

	recv := <-ch1
	assert.Equal(t, val, recv)

	// Second subscriber - first should also receive the value
	ch2, err := ps.Subscribe(ctx, topic)
	assert.NoError(t, err)
	assert.NotNil(t, ch2)

	err = ps.Publish(ctx, topic, val)
	assert.NoError(t, err)

	recv = <-ch1
	assert.Equal(t, val, recv)
	recv = <-ch2
	assert.Equal(t, val, recv)

	// Closing the pubsub instance should close all the subscribers
	err = ps.Close()
	assert.NoError(t, err)

	recv = <-ch1
	assert.Equal(t, pubsub.ClosingValue, recv)
	recv = <-ch2
	assert.Equal(t, pubsub.ClosingValue, recv)
}

func TestMemoryPubsubUnsubscribe(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ps := NewMemoryPubsub(t)
	topic := "test"

	ch, err := ps.Subscribe(ctx, topic)
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	val := "publish val"
	err = ps.Publish(ctx, topic, val)
	assert.NoError(t, err)
	recv := <-ch
	assert.Equal(t, val, recv)

	err = ps.Unsubscribe(ctx, ch)
	assert.NoError(t, err)

	err = ps.Publish(ctx, topic, val)
	assert.NoError(t, err)
	recv = <-ch
	assert.Equal(t, pubsub.ClosingValue, recv) // we shouldn't see 'val' this time
}
