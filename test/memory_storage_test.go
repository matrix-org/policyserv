package test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
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

func TestMemoryStorageEduTransactions(t *testing.T) {
	t.Parallel()

	// This test is verifying that MemoryStorage behaves according to the PersistentStorage interface

	s := NewMemoryStorage(t)

	destination := "example.org"

	// Insert 105 EDUs into the table. A Matrix transaction should only contain 100 max
	for i := 0; i < 105; i++ {
		assert.NoError(t, s.InsertEdu(context.Background(), &storage.StoredEdu{
			Destination: destination,
			Payload: gomatrixserverlib.EDU{
				Type:    "org.matrix.policyserv.doesnt_matter",
				Content: []byte(fmt.Sprintf(`{"key": %d}`, i)),
			},
		}))
	}

	// Start a Matrix transaction, but don't commit it right away. We're going to test the locking.
	mxTxn, sqlTxn, err := s.BeginMatrixTransaction(context.Background(), destination)
	assert.NoError(t, err)
	assert.NotNil(t, mxTxn)
	assert.NotNil(t, sqlTxn)
	assert.Equal(t, 100, len(mxTxn.Edus))

	// Try to get a second transaction, which should encounter a lock
	gotTxn := false
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		mxTxn2, sqlTxn2, err2 := s.BeginMatrixTransaction(context.Background(), destination)
		gotTxn = true
		defer sqlTxn2.Rollback()

		// These asserts are for when the transaction unlocks
		assert.NoError(t, err2)
		assert.NotNil(t, mxTxn2)
		assert.NotNil(t, sqlTxn2)
		assert.Equal(t, 100, len(mxTxn2.Edus))
	}()

	// Wait a bit to see if the transaction got picked up
	time.Sleep(100 * time.Millisecond)
	assert.False(t, gotTxn)

	// Now, roll back the first transaction to unblock the second one
	assert.NoError(t, sqlTxn.Rollback())
	wg.Wait() // wait for the second transaction to finish rolling back itself

	// Both transactions should have been rolled back now. Now we're going to test that transactions return EDUs (or
	// sql.ErrNoRows) even when their locks overlap. We also test that inserting an EDU is picked up in the next transaction.
	mxTxn, sqlTxn, err = s.BeginMatrixTransaction(context.Background(), destination)
	assert.NoError(t, err)
	assert.NotNil(t, mxTxn)
	assert.NotNil(t, sqlTxn)
	assert.Equal(t, 100, len(mxTxn.Edus))

	// Insert an EDU into the table
	assert.NoError(t, s.InsertEdu(context.Background(), &storage.StoredEdu{
		Destination: destination,
		Payload: gomatrixserverlib.EDU{
			Type:    "org.matrix.policyserv.doesnt_matter",
			Content: []byte(`{"key": 105}`), // zero indexed means this is the 106th EDU
		},
	}))

	// Grab the second (and third) transactions in a gofunc because locking
	lastTxnWg := &sync.WaitGroup{}
	lastTxnWg.Add(2)
	gotTxn = false
	go func() {
		defer lastTxnWg.Done()
		mxTxn2, sqlTxn2, err2 := s.BeginMatrixTransaction(context.Background(), destination)
		gotTxn = true

		// These asserts are for when the transaction unlocks
		assert.NoError(t, err2)
		assert.NotNil(t, mxTxn2)
		assert.NotNil(t, sqlTxn2)
		assert.Equal(t, 6, len(mxTxn2.Edus)) // 5 from the initial insert and 1 from the insert a couple lines up

		// Start that third transaction, which should get an sql.ErrNoRows
		gotTxn = false
		go func() {
			defer lastTxnWg.Done()
			mxTxn3, sqlTxn3, err3 := s.BeginMatrixTransaction(context.Background(), destination)
			gotTxn = true

			// This assert is for when the transaction unlocks
			assert.Equal(t, sql.ErrNoRows, err3)
			assert.Nil(t, mxTxn3)
			assert.Nil(t, sqlTxn3)
		}()

		// Wait to see if the third transaction got picked up
		time.Sleep(100 * time.Millisecond)
		assert.False(t, gotTxn)
		assert.NoError(t, sqlTxn2.Commit()) // unlock the third transaction
	}()

	// Wait to see if the second transaction got picked up
	time.Sleep(100 * time.Millisecond)
	assert.False(t, gotTxn)
	assert.NoError(t, sqlTxn.Commit()) // unlock the second transaction

	lastTxnWg.Wait() // wait for the test to complete
}
