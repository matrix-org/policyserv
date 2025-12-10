package test

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"golang.org/x/crypto/ed25519"
)

type BaseClientEvent struct {
	RoomId      string         `json:"room_id"`
	EventId     string         `json:"event_id"`
	Type        string         `json:"type"`
	StateKey    *string        `json:"state_key"`
	Sender      string         `json:"sender"`
	Content     map[string]any `json:"content"`
	StickyUntil time.Time      `json:"-"` // exclude from JSON to avoid making the event improper/too large
}

func MustMakePDU(event *BaseClientEvent) gomatrixserverlib.PDU {
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

func (n *noopPDU) JSON() []byte {
	b, _ := json.Marshal(n.base)
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

// ----- below here are template functions -----

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

func (n *noopPDU) PrevEventIDs() []string {
	return make([]string, 0)
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

func (n *noopPDU) Sign(signingName string, keyID gomatrixserverlib.KeyID, privateKey ed25519.PrivateKey) gomatrixserverlib.PDU {
	return nil
}

func (n *noopPDU) Depth() int64 {
	return 0
}

func (n *noopPDU) AuthEventIDs() []string {
	return make([]string, 0)
}

func (n *noopPDU) ToHeaderedJSON() ([]byte, error) {
	return nil, errors.New("unsupported")
}
