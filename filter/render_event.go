package filter

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/matrix-org/gomatrixserverlib"
)

type messageEventContent struct {
	Body          string `json:"body"`
	Msgtype       string `json:"msgtype"`
	FormattedBody string `json:"formatted_body"`
	//Format string `json:"format"` // we don't really care what the format is
}

type reactionEventContent struct {
	RelatesTo struct {
		Key string `json:"key"`
		// other fields are part of a relationship, but we aren't concerned about them
	} `json:"m.relates_to"`
}

func renderEventToText(event gomatrixserverlib.PDU) ([]string, error) {
	switch event.Type() {
	case "m.room.message":
		content := &messageEventContent{}
		err := json.Unmarshal(event.Content(), &content)
		if err != nil {
			return nil, err
		}
		if !slices.Contains([]string{"m.text", "m.notice", "m.emote"}, content.Msgtype) {
			return nil, nil
		}
		prefix := fmt.Sprintf("%s says: ", event.SenderID())
		if content.Msgtype == "m.emote" {
			prefix = fmt.Sprintf("%s says: /me ", event.SenderID())
		}
		renders := []string{prefix + content.Body}
		if content.FormattedBody != "" {
			renders = append(renders, prefix+content.FormattedBody)
		}
		return renders, nil
	case "m.reaction":
		content := &reactionEventContent{}
		err := json.Unmarshal(event.Content(), &content)
		if err != nil {
			return nil, err
		}
		if content.RelatesTo.Key == "" {
			return nil, nil
		}
		return []string{fmt.Sprintf("%s reacted with %s", event.SenderID(), content.RelatesTo.Key)}, nil
	}
	return nil, nil // not renderable
}
