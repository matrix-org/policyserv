package test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/matrix-org/policyserv/storage"
	"github.com/stretchr/testify/assert"
)

// We want to make sure our test fixture logic is accurate too

func TestMemoryStorageReturnsEventErrorResultOnId(t *testing.T) {
	s := NewMemoryStorage(t)
	res, err := s.GetEventResult(context.Background(), ErrorEventResultId)
	assert.Nil(t, res)
	assert.Equal(t, SimulatedError, err)
}

func TestMemoryStorageGlobMatching(t *testing.T) {
	roomId := "!example"
	s := NewMemoryStorage(t)
	err := s.SetListBanRules(context.Background(), roomId, map[string]string{
		"@alice*:*":     "m.policy.rule.user",
		"*.example.org": "m.policy.rule.server",
	})
	assert.NoError(t, err)

	assertBannedState := func(userId string, shouldBeBanned bool) {
		banned, err := s.IsUserBannedInList(context.Background(), roomId, userId)
		assert.NoError(t, err)
		assert.Equal(t, shouldBeBanned, banned, fmt.Sprintf("expected ban state %t for %s", shouldBeBanned, userId))
	}

	assertBannedState("@alice:example.org", true)
	assertBannedState("@aliceeeeee:example.org", true)
	assertBannedState("@alice:subdomain.example.org", true)
	assertBannedState("@bob:subdomain.example.org", true)
	assertBannedState("@bob:example.org", false)
}

func TestMemoryStorageStateLearnQueue(t *testing.T) {
	item1 := &storage.StateLearnQueueItem{
		RoomId:    "!room1",
		AtEventId: "$event1",
		ViaServer: "one.example.org",
	}
	item2 := &storage.StateLearnQueueItem{
		RoomId:    "!room2",
		AtEventId: "$event2",
		ViaServer: "two.example.org",
	}
	s := NewMemoryStorage(t)

	// First the easy test: add two items, pop them out in order, then check that the queue is empty
	err := s.PushStateLearnQueue(context.Background(), item1)
	assert.NoError(t, err)
	err = s.PushStateLearnQueue(context.Background(), item2)
	assert.NoError(t, err)

	ret, txn, err := s.PopStateLearnQueue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, item1, ret)
	assert.NoError(t, txn.Commit())
	ret, txn, err = s.PopStateLearnQueue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, item2, ret)
	assert.NoError(t, txn.Commit())

	ret, txn, err = s.PopStateLearnQueue(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, ret)
	assert.Nil(t, txn)

	// Next the harder test: add two items, pop them out at the same time (don't commit them), and check that the queue is empty
	err = s.PushStateLearnQueue(context.Background(), item1)
	assert.NoError(t, err)
	err = s.PushStateLearnQueue(context.Background(), item2)
	assert.NoError(t, err)

	ret1, txn1, err := s.PopStateLearnQueue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, item1, ret1)
	assert.NotNil(t, txn1)
	ret2, txn2, err := s.PopStateLearnQueue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, item2, ret2)
	assert.NotNil(t, txn2)
	ret3, txn3, err := s.PopStateLearnQueue(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, ret3)
	assert.Nil(t, txn3)

	// Now that we're in an empty-but-nothing-committed state, roll back txn1 and then pop it again to ensure that works
	assert.NoError(t, txn1.Rollback())
	ret4, txn4, err := s.PopStateLearnQueue(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, item1, ret4)
	assert.NotNil(t, txn4)

	// Finally, commit txn2 and ensure that it's gone
	assert.NoError(t, txn2.Commit())
	ret5, txn5, err := s.PopStateLearnQueue(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, ret5)
	assert.Nil(t, txn5)
}

func TestMemoryStorageTrustData(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage(t)

	res := struct {
		Data string `json:"data"`
	}{}

	// Set some data
	res.Data = "hello world"
	err := s.SetTrustData(context.Background(), "x", "x", res)
	assert.NoError(t, err)

	// Source and key found
	res.Data = "reset for testing" // if we don't do this, we aren't testing that the value is actually updated
	err = s.GetTrustData(context.Background(), "x", "x", &res)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", res.Data)

	// Source not found
	err = s.GetTrustData(context.Background(), "different", "x", &res)
	assert.Equal(t, sql.ErrNoRows, err)

	// Source found, key not found
	err = s.GetTrustData(context.Background(), "x", "different", &res)
	assert.Equal(t, sql.ErrNoRows, err)
}
