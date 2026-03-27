package homeserver

import (
	"context"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestSendRedactInstructionUnknownRoom(t *testing.T) {
	t.Parallel()

	hs := NewMockServer(t, NoConfigChanges)

	pdu := test.MustMakePDU(&test.BaseClientEvent{
		Type:   "m.room.message",
		RoomId: "!room:example.org",
		Content: map[string]any{
			"body": "hello world",
		},
	})

	// We don't insert the room or community into the database, so the redaction code should treat this
	// as "unknown".

	err := hs.SendRedactInstruction(context.Background(), pdu)
	assert.NoError(t, err)

	// If an EDU was inserted, it'll cause the destination to "need catchup".
	catchupDestinations, err := hs.storage.GetDestinationsNeedingCatchup(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, len(catchupDestinations))
}

func TestSendRedactInstructionUnknownCommunity(t *testing.T) {
	t.Parallel()

	hs := NewMockServer(t, NoConfigChanges)

	pdu := test.MustMakePDU(&test.BaseClientEvent{
		Type:   "m.room.message",
		RoomId: "!room:example.org",
		Content: map[string]any{
			"body": "hello world",
		},
	})

	err := hs.storage.UpsertRoom(context.Background(), &storage.StoredRoom{
		RoomId:      pdu.RoomID().String(),
		RoomVersion: "10",
		CommunityId: "default",
	})
	assert.NoError(t, err)

	// We don't insert the community into the database, so the redaction code should treat this
	// as "unknown".

	err = hs.SendRedactInstruction(context.Background(), pdu)
	assert.NoError(t, err)

	// If an EDU was inserted, it'll cause the destination to "need catchup".
	catchupDestinations, err := hs.storage.GetDestinationsNeedingCatchup(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, len(catchupDestinations))
}

func TestSendRedactInstructionCommunityNoModerationBot(t *testing.T) {
	t.Parallel()

	hs := NewMockServer(t, NoConfigChanges)

	pdu := test.MustMakePDU(&test.BaseClientEvent{
		Type:   "m.room.message",
		RoomId: "!room:example.org",
		Content: map[string]any{
			"body": "hello world",
		},
	})

	err := hs.storage.UpsertRoom(context.Background(), &storage.StoredRoom{
		RoomId:      pdu.RoomID().String(),
		RoomVersion: "10",
		CommunityId: "default",
	})
	assert.NoError(t, err)

	err = hs.storage.UpsertCommunity(context.Background(), &storage.StoredCommunity{
		CommunityId: "default",
		Name:        "Testing",
		Config: &config.CommunityConfig{
			HellbanPostfilterMinutes: internal.Pointer(-1),
			ModerationBotUserId:      internal.Pointer(""), // no moderation bot
		},
	})

	err = hs.SendRedactInstruction(context.Background(), pdu)
	assert.NoError(t, err)

	// If an EDU was inserted, it'll cause the destination to "need catchup".
	catchupDestinations, err := hs.storage.GetDestinationsNeedingCatchup(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, len(catchupDestinations))
}

func TestSendRedactInstruction(t *testing.T) {
	t.Parallel()

	hs := NewMockServer(t, NoConfigChanges)

	pdu := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$redact_me",
		Type:    "m.room.message",
		RoomId:  "!room:example.org",
		Content: map[string]any{
			"body": "hello world",
		},
	})

	err := hs.storage.UpsertRoom(context.Background(), &storage.StoredRoom{
		RoomId:      pdu.RoomID().String(),
		RoomVersion: "10",
		CommunityId: "default",
	})
	assert.NoError(t, err)

	err = hs.storage.UpsertCommunity(context.Background(), &storage.StoredCommunity{
		CommunityId: "default",
		Name:        "Testing",
		Config: &config.CommunityConfig{
			HellbanPostfilterMinutes: internal.Pointer(-1),
			ModerationBotUserId:      internal.Pointer("@user:example.org"),
		},
	})

	err = hs.SendRedactInstruction(context.Background(), pdu)
	assert.NoError(t, err)

	// If an EDU was inserted, it'll cause the destination to "need catchup".
	catchupDestinations, err := hs.storage.GetDestinationsNeedingCatchup(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(catchupDestinations))

	// Get that EDU so we can verify it
	mxTxn, sqlTxn, err := hs.storage.BeginMatrixTransaction(context.Background(), catchupDestinations[0])
	assert.NoError(t, err)
	assert.NoError(t, sqlTxn.Commit())
	assert.Equal(t, 1, len(mxTxn.Edus))
	edu := mxTxn.Edus[0]

	// Verify the EDU shape, starting with the outer to-device carrier type
	assert.Equal(t, "m.direct_to_device", edu.Type)
	contentJson := string(edu.Content)
	assert.Equal(t, "org.matrix.policyserv.command", gjson.Get(contentJson, "type").String())
	assert.Equal(t, hs.localActor.String(), gjson.Get(contentJson, "sender").String())
	assert.NotEmpty(t, gjson.Get(contentJson, "message_id").String())
	messagesMap := gjson.Get(contentJson, "messages").Map()
	assert.Equal(t, 1, len(messagesMap))
	assert.Equal(t, 1, len(messagesMap["@user:example.org"].Map()))
	assert.NotNil(t, messagesMap["@user:example.org"].Map()["*"])

	// Now verify the actual message itself, including signature
	signedBodyRaw := messagesMap["@user:example.org"].Map()["*"]
	signedBody := signedBodyRaw.Map()
	assert.Equal(t, "redact", signedBody["command"].String())
	assert.Equal(t, pdu.RoomID().String(), signedBody["room_id"].String())
	assert.Equal(t, pdu.EventID(), signedBody["event_id"].String())
	err = gomatrixserverlib.VerifyJSON(string(hs.ServerName), PolicyServerKeyID, hs.GetPublicEventSigningKey(), []byte(signedBodyRaw.Raw))
	assert.NoError(t, err)
}
