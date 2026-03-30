package homeserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/sjson"
)

var shouldReturnEvent error = nil

func returnEventHandler(t *testing.T, hs *Homeserver, eventOrError func(reqCount int) error) (http.HandlerFunc, *int, gomatrixserverlib.PDU) {
	// Prepare an event that would be returned
	pdu := MakeSignedPDUForTest(t, hs, &test.BaseClientEvent{
		RoomId:  "!room:example.org",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{"body": "this is the body"},
	})

	responseCount := 0
	lock := &sync.Mutex{}
	return func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		responseCount++
		lock.Unlock()

		err := eventOrError(responseCount - 1) // -1 because we're already incremented
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"errcode":"M_UNKNOWN", "error":"%s"}`, err.Error())))
		} else {
			w.Header().Set("Content-Type", "application/json")

			// Per spec, this endpoint returns a single-PDU transaction
			txn, err := sjson.SetRawBytes([]byte(`{"pdus":[]}`), "pdus.0", pdu.JSON())
			assert.NoError(t, err) // "should never happen"
			_, _ = w.Write(txn)
		}
	}, &responseCount, pdu
}

// TestGetEventSlowFirstServer - Tests that requesting an event from multiple remote servers is not impeded by
// the first server being slow.
func TestGetEventSlowFirstServer(t *testing.T) {
	t.Parallel()

	hs := NewMockServerForTest(t, test.NewMemoryStorage(t), func(c *Config) {
		c.SkipVerify = true // our httptest server will have an unknown authority
	})

	// Set up a test server to act as our remote homeserver (event fetch server)
	handler, responseCount, pdu := returnEventHandler(t, hs, func(reqCount int) error {
		if reqCount < 1 {
			// we want to stall the "first" request made to the event fetch server to simulate it being
			// slow. This should cause the code to pick the second server's response instead.
			time.Sleep(1 * time.Second)
		}
		return shouldReturnEvent
	})
	localhost := httptest.NewTLSServer(handler)
	defer localhost.Close()
	parsed, err := url.Parse(localhost.URL)
	assert.NoError(t, err) // "should never happen"
	localhostPort := parsed.Port()

	// Prepare a known room
	err = hs.storage.UpsertRoom(context.Background(), &storage.StoredRoom{
		RoomId:      pdu.RoomID().String(),
		CommunityId: "default",
		RoomVersion: "10",
	})
	assert.NoError(t, err)

	// Now, we can test that we get the event
	event, err := hs.GetEvent(context.Background(), "$whatever", []string{
		fmt.Sprintf("127.0.0.1:%s", localhostPort),
		fmt.Sprintf("127.0.0.1:%s", localhostPort), // this should be the request that returns
	})
	assert.NoError(t, err)
	assert.NotNil(t, event)
	assert.False(t, event.Redacted()) // if it's redacted, something went wrong with the hashes
	assert.Equal(t, 2, *responseCount)

	// We don't check *all* event fields (especially because `.JSON()` on each expected and actual might
	// return different things), but we do check enough to ensure we got a useful event.
	assert.Equal(t, pdu.RoomID().String(), event.RoomID().String())
	assert.Equal(t, pdu.Type(), event.Type())
	assert.Equal(t, pdu.SenderID(), event.SenderID())
	assert.Equal(t, pdu.Content(), event.Content())
}

// TestGetEventErrorFirstServer - Like TestGetEventSlowFirstServer, but the first server errors out instead.
func TestGetEventErrorFirstServer(t *testing.T) {
	t.Parallel()

	hs := NewMockServerForTest(t, test.NewMemoryStorage(t), func(c *Config) {
		c.SkipVerify = true // our httptest server will have an unknown authority
	})

	// Set up a test server to act as our remote homeserver (event fetch server)
	handler, responseCount, pdu := returnEventHandler(t, hs, func(reqCount int) error {
		if reqCount < 1 {
			// Per test docs, we want this one to error instead of just being slow
			return errors.New("this error should be ignored")
		}
		return shouldReturnEvent
	})
	localhost := httptest.NewTLSServer(handler)
	defer localhost.Close()
	parsed, err := url.Parse(localhost.URL)
	assert.NoError(t, err) // "should never happen"
	localhostPort := parsed.Port()

	// Prepare a known room
	err = hs.storage.UpsertRoom(context.Background(), &storage.StoredRoom{
		RoomId:      pdu.RoomID().String(),
		CommunityId: "default",
		RoomVersion: "10",
	})
	assert.NoError(t, err)

	// Now, we can test that we get the event
	event, err := hs.GetEvent(context.Background(), "$whatever", []string{
		fmt.Sprintf("127.0.0.1:%s", localhostPort),
		fmt.Sprintf("127.0.0.1:%s", localhostPort), // this should be the request that returns
	})
	assert.NoError(t, err)
	assert.NotNil(t, event)
	assert.False(t, event.Redacted()) // if it's redacted, something went wrong with the hashes
	assert.Equal(t, 2, *responseCount)

	// We don't check *all* event fields (especially because `.JSON()` on each expected and actual might
	// return different things), but we do check enough to ensure we got a useful event.
	assert.Equal(t, pdu.RoomID().String(), event.RoomID().String())
	assert.Equal(t, pdu.Type(), event.Type())
	assert.Equal(t, pdu.SenderID(), event.SenderID())
	assert.Equal(t, pdu.Content(), event.Content())
}

// TestGetEventErrorAllServers - Tests that an error is returned if all remote servers fail.
func TestGetEventErrorAllServers(t *testing.T) {
	t.Parallel()

	hs := NewMockServerForTest(t, test.NewMemoryStorage(t), func(c *Config) {
		c.SkipVerify = true // our httptest server will have an unknown authority
	})

	// Set up a test server to act as our remote homeserver (event fetch server)
	handler, responseCount, _ := returnEventHandler(t, hs, func(reqCount int) error {
		return errors.New("this error should be returned")
	})
	localhost := httptest.NewTLSServer(handler)
	defer localhost.Close()
	parsed, err := url.Parse(localhost.URL)
	assert.NoError(t, err) // "should never happen"
	localhostPort := parsed.Port()

	// Now request the event (we don't need to prepare a room because we're never going to get an event)
	event, err := hs.GetEvent(context.Background(), "$whatever", []string{
		fmt.Sprintf("127.0.0.1:%s", localhostPort),
		fmt.Sprintf("127.0.0.1:%s", localhostPort), // this should be the request that returns
	})
	assert.ErrorContains(t, err, "this error should be returned")
	assert.Nil(t, event)
	assert.Equal(t, 2, *responseCount)
}
