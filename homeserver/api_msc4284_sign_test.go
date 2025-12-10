package homeserver

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/stretchr/testify/assert"
)

var (
	policyServerName spec.ServerName = "policy.server.test"
	otherServerName  spec.ServerName = "other.server.test"
)

func TestSignEvent(t *testing.T) {
	_, policyServerPrivKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)
	_, otherServerPrivKey, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)
	server := &Homeserver{
		ServerName:      policyServerName,
		eventSigningKey: policyServerPrivKey,
	}
	event, err := gomatrixserverlib.MustGetRoomVersion("10").NewEventBuilderFromProtoEvent(&gomatrixserverlib.ProtoEvent{
		SenderID: "@alice:" + string(otherServerName),
		RoomID:   "!foo:bar",
		Type:     "m.room.message",
		Content:  []byte(`{"msgtype":"m.text","body":"Hello world"}`),
	}).Build(time.Now(), otherServerName, gomatrixserverlib.KeyID("1"), otherServerPrivKey)
	assert.NoError(t, err)
	t.Logf("event to sign: %v", string(event.JSON()))
	w := httptest.NewRecorder()
	signEvent(server, event, w)
	respBody := w.Body.String()
	t.Logf("got signatures: %v", respBody)
	var sigs signatures
	assert.NoError(t, json.Unmarshal([]byte(respBody), &sigs.Signatures))
	assert.Equal(t, 1, len(sigs.Signatures))
	keyIDToSig, ok := sigs.Signatures[string(policyServerName)]
	assert.Equal(t, true, ok)
	assert.Equal(t, 1, len(keyIDToSig))
	sig, ok := keyIDToSig[PolicyServerKeyID]
	assert.Equal(t, true, ok)
	assert.Equal(t, true, len(sig) > 0)
}
