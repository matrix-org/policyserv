package homeserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type policySignTestCase struct {
	TestName string
	Event    gomatrixserverlib.PDU
	IsSpam   bool
}

func TestHttpPolicySign(t *testing.T) {
	t.Parallel()

	server := test.NewMockServer(t, test.NoConfigChanges)

	originName := "origin.example.org"
	roomId := "!foo:example.org"
	testCases := []policySignTestCase{
		{
			TestName: "spammy events are spammy",
			Event: test.MustMakePDU(&test.BaseClientEvent{
				// No EventID because that'll break signing the event
				RoomId: roomId,
				Type:   "m.room.message",
				Sender: "@spam:" + originName,
				Content: map[string]any{
					"body": "body doesn't matter",
				},
			}),
			IsSpam: true,
		},
		{
			TestName: "non-spammy events are not spammy",
			Event: test.MustMakePDU(&test.BaseClientEvent{
				// No EventID because that'll break signing the event
				RoomId: roomId,
				Type:   "m.room.message",
				Sender: "@not_spam:" + originName,
				Content: map[string]any{
					"body": "body doesn't matter",
				},
			}),
			IsSpam: false,
		},
	}

	err := server.storage.UpsertRoom(context.Background(), &storage.StoredRoom{
		RoomId:      roomId,
		CommunityId: "default",
		RoomVersion: "10",
	})
	assert.NoError(t, err)

	for _, tc := range testCases {
		fmt.Println("Test case: ", tc.TestName)

		// Configure the filter/community per the test case
		cnf := &config.CommunityConfig{
			// disable hellban to avoid it interfering with other test cases
			HellbanPostfilterMinutes: internal.Pointer(-1),
		}
		senderId := tc.Event.SenderID().ToUserID().String()
		if tc.IsSpam {
			cnf.KeywordFilterKeywords = &[]string{senderId}
			cnf.KeywordFilterUseFullEvent = internal.Pointer(true) // we want to pick up `sender` as spam
		} else {
			cnf.SenderPrefilterAllowedSenders = &[]string{senderId}
		}
		err = server.storage.UpsertCommunity(context.Background(), &storage.StoredCommunity{
			CommunityId: "default",
			Name:        "default",
			Config:      cnf,
		})
		assert.NoError(t, err)

		// Prepare and send a request
		res := httptest.NewRecorder()
		req := server.MustMakeFederationRequest(t, http.MethodPost, "/_matrix/policy/v1/sign", tc.Event, originName)

		// Grab a copy of what we're sending so we can use it later, if needed. The request body is the fully signed and
		// hashed event whereas `tc.Event` might not be totally valid, and we want the most valid thing we can get for later.
		bodyCopy, err := req.GetBody()
		assert.NoError(t, err, fmt.Sprintf("%s => request body should be clonable", tc.TestName))
		sentEvent, err := io.ReadAll(bodyCopy)
		assert.NoError(t, err, fmt.Sprintf("%s => request body should be readable", tc.TestName))

		httpPolicySign(server, res, req)
		resBody := res.Body.String()
		t.Logf("Response body: %d - %s", res.Code, resBody)

		// Assert response
		if tc.IsSpam {
			assert.Equal(t, http.StatusBadRequest, res.Code, fmt.Sprintf("%s => wanted 400 Bad Request", tc.TestName))
			assert.Equal(t, "application/json", res.Header().Get("Content-Type"), fmt.Sprintf("%s => wanted JSON response", tc.TestName))
			test.AssertApiError(t, res, "M_FORBIDDEN", "This message is not allowed by the policy server")
		} else {
			assert.Equal(t, http.StatusOK, res.Code, fmt.Sprintf("%s => wanted 200 OK", tc.TestName))
			assert.Equal(t, "application/json", res.Header().Get("Content-Type"), fmt.Sprintf("%s => wanted JSON response", tc.TestName))

			// Verify that a signature was in fact returned
			sigs := signatures{}
			err = json.Unmarshal([]byte(resBody), &sigs.Signatures)
			assert.NoError(t, err, fmt.Sprintf("%s => expected to unmarshal signatures", tc.TestName))
			assert.NotEmpty(t, sigs.Signatures, fmt.Sprintf("%s => expected to have signatures", tc.TestName))
			assert.Contains(t, sigs.Signatures, string(server.ServerName), fmt.Sprintf("%s => expected to have signature for %s", tc.TestName, server.ServerName))
			assert.Contains(t, sigs.Signatures[string(server.ServerName)], PolicyServerKeyID, fmt.Sprintf("%s => expected to have signature for %s", tc.TestName, PolicyServerKeyID))

			// Add that signature to the event (creating a new GMSL event)
			escapedServerName := strings.ReplaceAll(string(server.ServerName), ".", "\\.")
			signature := gjson.GetBytes([]byte(resBody), escapedServerName)
			withSignature, err := sjson.SetBytes(sentEvent, "signatures."+escapedServerName, signature.Value())
			signedEvent, err := gomatrixserverlib.MustGetRoomVersion("10").NewEventFromUntrustedJSON(withSignature)
			assert.NoError(t, err, fmt.Sprintf("%s => expected to parse signed event", tc.TestName))

			// Now validate the policy server's signature
			err = gomatrixserverlib.VerifyEventSignatures(context.Background(), signedEvent, server.keyRing, func(roomId spec.RoomID, senderId spec.SenderID) (*spec.UserID, error) {
				return senderId.ToUserID(), nil
			})
			assert.NoError(t, err, fmt.Sprintf("%s => event should be validly signed", tc.TestName))
		}
	}
}
