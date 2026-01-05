package homeserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/trust"
)

type displayNameOnly struct {
	DisplayName string `json:"displayname,omitempty"`
}

type minimalPolicyRule struct {
	Recommendation string `json:"recommendation,omitempty"`
	Entity         string `json:"entity,omitempty"`
}

func (h *Homeserver) shouldLearnState(ctx context.Context, roomId string) (bool, *storage.StoredRoom, error) {
	_, ok := h.stateLearnCache.Get(roomId)
	if ok {
		// we already requested a state learn update for this room, so don't ask for another one.
		return false, nil, nil
	}

	room, err := h.storage.GetRoom(ctx, roomId)
	if err != nil {
		return false, nil, err
	}
	if room == nil {
		return false, nil, fmt.Errorf("room %s not found", roomId)
	}

	if time.Now().Sub(time.UnixMilli(room.LastCachedStateTimestampMillis)) < h.cacheRoomStateFor {
		return false, room, nil // not expired
	}

	return true, room, nil
}

func (h *Homeserver) LearnStateIfExpired(ctx context.Context, roomId string, atEventId string, via string) error {
	shouldLearn, room, err := h.shouldLearnState(ctx, roomId)
	if err != nil {
		return err
	}
	if !shouldLearn {
		return nil
	}

	log.Printf("Fetching state for %s at %s from %s", roomId, atEventId, via)

	res, err := h.client.LookupState(ctx, h.ServerName, spec.ServerName(via), roomId, atEventId, gomatrixserverlib.RoomVersion(room.RoomVersion))
	if err != nil {
		return err
	}
	pdus := res.GetStateEvents().UntrustedEvents(gomatrixserverlib.RoomVersion(room.RoomVersion))

	// Scan all events for the types we're looking for
	userDisplayNames := make(map[string]string)
	entityBans := make(map[string]string)
	for _, pdu := range pdus {
		// Find display names
		if pdu.StateKeyEquals(string(pdu.SenderID())) && pdu.Type() == spec.MRoomMember {
			membership, err := pdu.Membership()
			if err != nil {
				return errors.Join(fmt.Errorf("error parsing membership for %s / %s / %s", pdu.SenderID(), pdu.EventID(), pdu.RoomID()), err)
			}
			if membership == spec.Join {
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
		}

		// Find policy rules
		if pdu.StateKey() != nil && len(*(pdu.StateKey())) > 0 && (pdu.Type() == "m.policy.rule.user" || pdu.Type() == "m.policy.rule.server") {
			content := minimalPolicyRule{}
			err = json.Unmarshal(pdu.Content(), &content)
			if err != nil {
				return errors.Join(fmt.Errorf("error parsing recommendation for %s / %s / %s", pdu.Type(), pdu.EventID(), pdu.RoomID()), err)
			}
			if content.Recommendation == "m.ban" && len(content.Entity) > 0 {
				entityBans[content.Entity] = pdu.Type()
			}
		}

		// Import all create events for sources
		if pdu.Type() == "m.room.create" && pdu.StateKey() != nil && *(pdu.StateKey()) == "" {
			log.Printf("Importing create event (%s) for %s", pdu.EventID(), roomId)
			source, err := trust.NewCreatorSource(h.storage)
			if err != nil {
				log.Printf("Non-fatal error creating creator source: %v", err)
			} else {
				err = source.ImportData(ctx, roomId, pdu)
				if err != nil {
					log.Printf("Non-fatal error importing creator data: %v", err)
				}
			}
		}

		// Import all power level events for sources
		if pdu.Type() == "m.room.power_levels" && pdu.StateKey() != nil && *(pdu.StateKey()) == "" {
			log.Printf("Importing power levels event (%s) for %s", pdu.EventID(), roomId)
			source, err := trust.NewPowerLevelsSource(h.storage)
			if err != nil {
				log.Printf("Non-fatal error creating power levels source: %v", err)
			} else {
				err = source.ImportData(ctx, roomId, pdu)
				if err != nil {
					log.Printf("Non-fatal error importing power levels data: %v", err)
				}
			}
		}
	}

	// Process the displaynames
	userIds := make([]string, 0)
	displayNames := make([]string, 0)
	for k, v := range userDisplayNames {
		userIds = append(userIds, k)
		displayNames = append(displayNames, v)
	}
	err = h.storage.SetUserIdsAndDisplayNamesByRoomId(ctx, roomId, userIds, displayNames)
	if err != nil {
		return errors.Join(fmt.Errorf("error storing displaynames for %s / %s", roomId, atEventId), err)
	}
	err = h.storage.SetListBanRules(ctx, roomId, entityBans)
	if err != nil {
		return errors.Join(fmt.Errorf("error storing list ban rules for %s / %s", roomId, atEventId), err)
	}

	// Bump the cache timestamp, fetching a fresh `room` in case something changed while we've been processing the state
	room, err = h.storage.GetRoom(ctx, roomId)
	if err != nil {
		return errors.Join(fmt.Errorf("error fetching room %s after processing state", roomId), err)
	}
	if room == nil {
		return fmt.Errorf("room %s not found after processing state", roomId)
	}
	room.LastCachedStateTimestampMillis = time.Now().UnixMilli()
	err = h.storage.UpsertRoom(ctx, room)
	if err != nil {
		return errors.Join(fmt.Errorf("error storing room %s after processing state", roomId), err)
	}

	return nil
}
