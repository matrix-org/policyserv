package homeserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/storage"
	"github.com/stretchr/testify/assert"
)

func TestSendNextTransactionTo(t *testing.T) {
	t.Parallel()

	hs := NewMockServer(t, func(c *Config) {
		c.SkipVerify = true // our httptest server will have an unknown authority
	})

	// We want to test a couple of conditions here:
	// 1. That the transaction is actually sent.
	// 2. That concurrent transactions yield exactly one request (ensure the singleflight works).
	// 3. That nothing is sent if there's nothing to send.
	//
	// To do this, we'll have to do some slightly awkward locking. We use a wait group to
	// prevent the transaction from fully sending the first time so we can ensure we get
	// a duplicate call in.

	txnWg := &sync.WaitGroup{}
	txnWg.Add(2) // we're going to block until both calls to `SendNextTransactionTo` have been made

	// We also need a wait group to understand when the transaction functions have finished
	doneWg := &sync.WaitGroup{}
	doneWg.Add(2)

	// Now we create our httptest server to act as the remote end
	sendCount := 0
	expectedEdu := gomatrixserverlib.EDU{
		Type:    "org.example.edu",
		Origin:  string(hs.ServerName),
		Content: []byte(`{"key":"value"}`),
	}
	localhost := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		time.Sleep(100 * time.Millisecond) // we add a small delay to eliminate race conditions (explained later)

		txnWg.Wait() // wait for all transaction requests to be made

		// Parse the transaction and verify it
		txn := &gomatrixserverlib.Transaction{}
		b, err := io.ReadAll(r.Body)
		assert.NoError(t, err) // "should never happen"
		err = json.Unmarshal(b, txn)
		assert.NoError(t, err) // "should never happen"
		assert.Equal(t, 1, len(txn.EDUs))
		assert.Equal(t, 0, len(txn.PDUs))
		assert.Equal(t, expectedEdu, txn.EDUs[0])

		// Reply 200 OK
		_, _ = w.Write([]byte(`{"pdus": {}}`))
	}))
	defer localhost.Close()
	parsed, err := url.Parse(localhost.URL)
	assert.NoError(t, err) // "should never happen"
	localhostName := fmt.Sprintf("127.0.0.1:%s", parsed.Port())
	expectedEdu.Destination = localhostName

	// Add the EDU to the queue
	err = hs.storage.InsertEdu(context.Background(), &storage.StoredEdu{
		Destination: localhostName,
		Payload:     expectedEdu,
	})
	assert.NoError(t, err)

	// Make both requests to actually send the transaction to our remote server. This is where gofuncs
	// get a little complicated: `SendNextTransactionTo` blocks, which is desirable for the `doneWg` to
	// indicate when we've finished all of the requests. However, we don't want to block `txnWg` from
	// happening, but it does need to be after we start the transaction, otherwise we'll unblock the
	// httptest server too soon (leading to us testing the "nothing to send" condition rather than the
	// singleflight). There's a small chance that the gofunc doesn't *actually* run before `txnWg.Done()`
	// though, so the httptest server sleeps for a bit to give us a higher chance at testing the
	// singleflight.
	for i := 0; i < 2; i++ {
		go func() {
			go func() {
				hs.SendNextTransactionTo(context.Background(), localhostName)
				doneWg.Done()
			}()
			txnWg.Done()
		}()
	}

	// Wait for the requests to happen, then verify the request count that actually made it over the
	// wire. Note that this can't actually determine if the singleflight did its job or if the "nothing
	// to send" code path was triggered. We are hoping that we've given the singleflight a fighting
	// chance up above.
	doneWg.Wait()
	assert.Equal(t, 1, sendCount)

	// Now test the "nothing to send" case (the queue should be empty - we just sent everything)
	hs.SendNextTransactionTo(context.Background(), localhostName) // reminder: this blocks
	assert.Equal(t, 1, sendCount)
}

func TestSendNextTransactionToWithError(t *testing.T) {
	t.Parallel()

	hs := NewMockServer(t, func(c *Config) {
		c.SkipVerify = true // our httptest server will have an unknown authority
	})

	// We just want to ensure that `SendNextTransactionTo` rolls back upon error from the remote end.

	sendCount := 0
	expectedEdu := gomatrixserverlib.EDU{
		Type:    "org.example.edu",
		Origin:  string(hs.ServerName),
		Content: []byte(`{"key":"value"}`),
	}
	localhost := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++

		// Parse the transaction and verify it
		txn := &gomatrixserverlib.Transaction{}
		b, err := io.ReadAll(r.Body)
		assert.NoError(t, err) // "should never happen"
		err = json.Unmarshal(b, txn)
		assert.NoError(t, err) // "should never happen"
		assert.Equal(t, 1, len(txn.EDUs))
		assert.Equal(t, 0, len(txn.PDUs))
		assert.Equal(t, expectedEdu, txn.EDUs[0])

		// Reply with error
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errcode":"M_UNKNOWN","error":"Something went wrong"}`))
	}))
	defer localhost.Close()
	parsed, err := url.Parse(localhost.URL)
	assert.NoError(t, err) // "should never happen"
	localhostName := fmt.Sprintf("127.0.0.1:%s", parsed.Port())
	expectedEdu.Destination = localhostName

	// Add the EDU to the queue
	err = hs.storage.InsertEdu(context.Background(), &storage.StoredEdu{
		Destination: localhostName,
		Payload:     expectedEdu,
	})
	assert.NoError(t, err)

	// Test that errors cause a re-queue
	hs.SendNextTransactionTo(context.Background(), localhostName)
	assert.Equal(t, 1, sendCount) // first request should have happened, and should fail
	hs.SendNextTransactionTo(context.Background(), localhostName)
	assert.Equal(t, 2, sendCount) // it should have re-queued the EDU for sending
}
