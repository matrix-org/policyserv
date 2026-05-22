package learning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/storage"
)

type RoomMembersLearner struct {
	storage storage.PersistentStorage
}

func (r *RoomMembersLearner) CanLearn(ctx context.Context, room *storage.StoredRoom, event gomatrixserverlib.PDU) (bool, error) {
	if event.Type() != "m.room.member" {
		return false, nil // not a member event
	}
	if event.StateKey() == nil {
		return false, nil // not a state event
	}
	if !event.StateKeyEquals(string(event.SenderID())) {
		return false, nil // probably not a join event
	}
	membership, err := event.Membership()
	if err != nil {
		return false, err // "should never happen"
	}
	if membership != spec.Join {
		return false, nil // not a join event
	}
	return true, nil
}

type displayNameOnly struct {
	DisplayName string `json:"displayname,omitempty"`
}

func (r *RoomMembersLearner) LearnFrom(ctx context.Context, room *storage.StoredRoom, roomState []gomatrixserverlib.PDU) error {
	userDisplayNames := make(map[string]string)
	for _, pdu := range roomState {
		ok, err := r.CanLearn(ctx, room, pdu)
		if err != nil {
			return err
		}
		if !ok {
			continue // not an event we care about
		}

		// Pull out the display name for mentions detection
		content := displayNameOnly{}
		err = json.Unmarshal(pdu.Content(), &content)
		if err != nil {
			return errors.Join(fmt.Errorf("error parsing displayname for %s / %s / %s", pdu.SenderID(), pdu.EventID(), pdu.RoomID()), err)
		}
		trimmed := strings.TrimSpace(content.DisplayName)
		if len(trimmed) > 0 {
			userDisplayNames[string(pdu.SenderID())] = content.DisplayName
		}
	}

	// Process the display names and user IDs which are joined in the room
	userIds := make([]string, 0)
	displayNames := make([]string, 0)
	for k, v := range userDisplayNames {
		userIds = append(userIds, k)
		displayNames = append(displayNames, v)
	}
	err := r.storage.SetUserIdsAndDisplayNamesByRoomId(ctx, room.RoomId, userIds, displayNames)
	if err != nil {
		return errors.Join(fmt.Errorf("error storing displaynames for %s", room.RoomId), err)
	}

	return nil
}
