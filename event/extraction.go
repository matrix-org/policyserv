package event

import (
	"encoding/json"

	"github.com/matrix-org/gomatrixserverlib"
)

// ExtractHtmlRepresentations extracts all HTML representations from a Matrix event.
// **Note**: This doesn't currently process extensible events, but in future when it does it may return more than
// one HTML representation. Callers should not assume that this only ever returns zero or one HTML representation.
// Returns nil if the event has no HTML representations.
func ExtractHtmlRepresentations(event gomatrixserverlib.PDU) ([]string, error) {
	if event.Type() != "m.room.message" {
		return nil, nil
	}

	content := &MessageEventContent{}
	err := json.Unmarshal(event.Content(), &content)
	if err != nil {
		return nil, err
	}

	if content.FormattedBody != "" {
		return []string{content.FormattedBody}, nil
	}

	return nil, nil
}
