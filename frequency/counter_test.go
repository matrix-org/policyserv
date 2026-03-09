package frequency

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestCounterLongName(t *testing.T) {
	t.Parallel()

	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	c, err := NewCounter(ps, strings.Repeat("x", 32), 60*time.Second)
	assert.Nil(t, c)
	assert.ErrorContains(t, err, "name must be less than 31 characters")
}

func TestCounter(t *testing.T) {
	t.Parallel()

	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	c, err := NewCounter(ps, "TestCounter_1", 60*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	defer c.Close()

	// Happy path test: increment a couple entities, get the rate out
	user1 := "@user1:example.org"
	user2 := "@user2:example.org"
	err = c.Increment(user1)
	assert.NoError(t, err)
	err = c.Increment(user1)
	assert.NoError(t, err)
	err = c.Increment(user2)
	assert.NoError(t, err)

	// Give it a moment to settle
	time.Sleep(250 * time.Millisecond)

	// Assert that the rate is correct
	rate, err := c.Get(user1)
	assert.NoError(t, err)
	assert.Equal(t, 2, rate)
	rate, err = c.Get(user2)
	assert.NoError(t, err)
	assert.Equal(t, 1, rate)
}

func TestCounterOldEntries(t *testing.T) {
	t.Parallel()

	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	c, err := NewCounter(ps, "TestCounter_2", 1000*time.Millisecond) // short window to avoid long test delays
	assert.NoError(t, err)
	assert.NotNil(t, c)
	defer c.Close()

	entity := "@user1:example.org"
	for i := 0; i < 10; i++ {
		err = c.Increment(entity)
		assert.NoError(t, err)
	}

	// Allow the entries to settle before we test that they actually made it (1 key and 10 entries)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, len(c.values))
	assert.Equal(t, 10, len(c.values[entity]))

	// Wait for the entries to expire/exceed the window (with some buffer)
	time.Sleep(1100 * time.Millisecond)

	// Verify the rate is zero as a result
	rate, err := c.Get(entity)
	assert.NoError(t, err)
	assert.Equal(t, 0, rate)
}

func manualIncrement(t *testing.T, c *Counter, entity string, ts time.Time) {
	// XXX: It's not great that we inject this way, but it does effectively test that the counter receives data
	// over the wire properly.
	r := record{
		Entity:    entity,
		Timestamp: ts,
	}
	b, err := json.Marshal(r)
	assert.NoError(t, err)
	err = c.pubsub.Publish(context.Background(), c.pubsubId, string(b))
	assert.NoError(t, err)
}

func TestCounterNewEntries(t *testing.T) {
	t.Parallel()

	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	c, err := NewCounter(ps, "TestCounter_3", 60*time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	defer c.Close()

	// Publish a timestamp in the future, simulating a host with a bad clock
	ts := time.Now().Add(5 * time.Second)
	entity := "@user1:example.org"
	manualIncrement(t, c, entity, ts)

	// Give it a moment to settle
	time.Sleep(250 * time.Millisecond)

	// Verify the rate is correct (we consider the "future" to be "now")
	rate, err := c.Get(entity)
	assert.NoError(t, err)
	assert.Equal(t, 1, rate)
}

func TestCounterCleanup(t *testing.T) {
	t.Parallel()

	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	c, err := NewCounter(ps, "TestCounter_4", 500*time.Millisecond) // short window to avoid long test delays
	assert.NoError(t, err)
	assert.NotNil(t, c)
	defer c.Close()

	// Publish a bunch of data (past, present, and future timestamps)
	manualIncrement(t, c, "@user1:example.org", time.Now().Add(-1500*time.Millisecond))
	manualIncrement(t, c, "@user2:example.org", time.Now())
	manualIncrement(t, c, "@user3:example.org", time.Now().Add(1500*time.Millisecond))

	// Allow the entries to settle before we test that they actually made it
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 3, len(c.values))

	// Wait for the first cleanup job to run (with some buffer)
	time.Sleep(1250 * time.Millisecond)

	// We should just have user3 values left
	assert.Equal(t, 1, len(c.values))
	assert.Contains(t, c.values, "@user3:example.org")

	// Now wait for the second cleanup job to run (with some buffer)
	time.Sleep(1250 * time.Millisecond)

	// We should have no values left
	assert.Equal(t, 0, len(c.values))
}
