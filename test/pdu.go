package test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/tidwall/sjson"
)

type BaseClientEvent struct {
	RoomId      string                       `json:"room_id"`
	EventId     string                       `json:"event_id,omitempty"`
	Type        string                       `json:"type"`
	StateKey    *string                      `json:"state_key"`
	Sender      string                       `json:"sender"`
	Content     map[string]any               `json:"content"`
	StickyUntil time.Time                    `json:"-"` // exclude from JSON to avoid making the event improper/too large
	Signatures  map[string]map[string]string `json:"signatures,omitempty"`

	// Things required to make GMSL happy (but we don't use them)
	AuthEventIds []string          `json:"auth_events"`
	PrevEventIds []string          `json:"prev_events"`
	Hashes       map[string]string `json:"hashes"`
}

func MustMakePDU(event *BaseClientEvent) gomatrixserverlib.PDU {
	// We need these values to have something in them to make GMSL happy
	if event.AuthEventIds == nil {
		event.AuthEventIds = []string{"$auth1"}
	}
	if event.PrevEventIds == nil {
		event.PrevEventIds = []string{"$prev1"}
	}
	if len(event.Hashes) == 0 {
		// https://spec.matrix.org/v1.17/server-server-api/#calculating-the-content-hash-for-an-event
		b, _ := json.Marshal(event)
		canonical, _ := gomatrixserverlib.CanonicalJSON(b)
		canonical, _ = sjson.DeleteBytes(canonical, "unsigned")
		canonical, _ = sjson.DeleteBytes(canonical, "signatures")
		canonical, _ = sjson.DeleteBytes(canonical, "hashes")
		canonical, _ = sjson.DeleteBytes(canonical, "event_id") // because it's not supposed to be there anyway
		hash := sha256.New()
		hash.Write(canonical)
		event.Hashes = map[string]string{
			"sha256": spec.Base64Bytes(hash.Sum(nil)).Encode(),
		}
	}
	return &noopPDU{base: event}
}

type noopPDU struct {
	base *BaseClientEvent
}

func (n *noopPDU) RoomID() spec.RoomID {
	i, err := spec.NewRoomID(n.base.RoomId)
	if err != nil {
		panic(err)
	}
	return *i
}

func (n *noopPDU) EventID() string {
	return n.base.EventId
}

func (n *noopPDU) Type() string {
	return n.base.Type
}

func (n *noopPDU) Content() []byte {
	b, _ := json.Marshal(n.base.Content)
	return b
}

func (n *noopPDU) StateKey() *string {
	return n.base.StateKey
}

func (n *noopPDU) StateKeyEquals(s string) bool {
	if n.base.StateKey == nil {
		return false
	}
	return *n.base.StateKey == s
}

func (n *noopPDU) SenderID() spec.SenderID {
	if n.base.Sender == "" {
		return spec.SenderIDFromUserID(spec.NewUserIDOrPanic("@unset_user_id:example.org", false))
	}
	return spec.SenderIDFromUserID(spec.NewUserIDOrPanic(n.base.Sender, false))
}

func (n *noopPDU) MarshalJSON() ([]byte, error) {
	// This function is needed to make `json.Marshal` work on `noopPDU` directly
	return json.Marshal(n.base)
}

func (n *noopPDU) JSON() []byte {
	b, _ := n.MarshalJSON()
	return b
}

func (n *noopPDU) IsSticky(now time.Time, received time.Time) bool {
	if n.base.StickyUntil.IsZero() {
		return false
	}
	return n.base.StickyUntil.After(now) && n.base.StickyUntil.After(received)
}

func (n *noopPDU) StickyEndTime(received time.Time) time.Time {
	if n.base.StickyUntil.IsZero() {
		return time.Time{} // zero, not sticky
	}
	return n.base.StickyUntil
}

func (n *noopPDU) Sign(signingName string, keyID gomatrixserverlib.KeyID, privateKey ed25519.PrivateKey) gomatrixserverlib.PDU {
	// Get the room version and redact the event accordingly. Note that in testing we hardcode the room version, so it
	// shouldn't fail.
	ver, err := gomatrixserverlib.GetRoomVersion(n.Version())
	if err != nil {
		panic(err) // "should never happen"
	}
	redacted, err := ver.RedactEventJSON(n.JSON())
	if err != nil {
		panic(err) // "should never happen"
	}

	// Get a copy of our "event" that's signed with the given key.
	signed, err := gomatrixserverlib.SignJSON(signingName, keyID, privateKey, redacted)
	if err != nil {
		panic(err) // "should never happen"
	}

	// Extract the signatures object and copy it into our base event
	base := &BaseClientEvent{}
	err = json.Unmarshal(signed, &base)
	if err != nil {
		panic(err) // "should never happen"
	}
	n.base.Signatures = base.Signatures // copy the signatures

	// We're supposed to clone the event according to the interface, but we aren't worried about mutation here.
	return n
}

func (n *noopPDU) AuthEventIDs() []string {
	return n.base.AuthEventIds
}

func (n *noopPDU) PrevEventIDs() []string {
	return n.base.PrevEventIds
}

// ----- below here are template functions -----

func (n *noopPDU) PrevStateEventIDs() []string {
	return make([]string, 0)
}

func (n *noopPDU) Version() gomatrixserverlib.RoomVersion {
	return gomatrixserverlib.RoomVersionV12
}

func (n *noopPDU) JoinRule() (string, error) {
	return "", errors.New("wrong event type")
}

func (n *noopPDU) HistoryVisibility() (gomatrixserverlib.HistoryVisibility, error) {
	return "", errors.New("wrong event type")
}

func (n *noopPDU) Membership() (string, error) {
	return "", errors.New("wrong event type")
}

func (n *noopPDU) PowerLevels() (*gomatrixserverlib.PowerLevelContent, error) {
	return nil, errors.New("wrong event type")
}

func (n *noopPDU) Redacts() string {
	return ""
}

func (n *noopPDU) Redacted() bool {
	return false
}

func (n *noopPDU) OriginServerTS() spec.Timestamp {
	return spec.AsTimestamp(time.Now())
}

func (n *noopPDU) Redact() {
	return
}

func (n *noopPDU) Unsigned() []byte {
	return []byte("{}")
}

func (n *noopPDU) SetUnsigned(unsigned interface{}) (gomatrixserverlib.PDU, error) {
	return nil, errors.New("unsupported")
}

func (n *noopPDU) SetUnsignedField(path string, value interface{}) error {
	return errors.New("unsupported")
}

func (n *noopPDU) Depth() int64 {
	return 0
}

func (n *noopPDU) ToHeaderedJSON() ([]byte, error) {
	return nil, errors.New("unsupported")
}
