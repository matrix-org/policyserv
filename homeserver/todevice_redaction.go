package homeserver

import (
	"context"
	"encoding/json"
	"log"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/redaction"
	"github.com/matrix-org/policyserv/storage"
)

func (h *Homeserver) SendRedactInstruction(ctx context.Context, forEvent gomatrixserverlib.PDU) error {
	log.Printf("[%s | %s] Redaction command requested for event", forEvent.EventID(), forEvent.RoomID().String())

	// Backwards compatibility first:
	err := redaction.QueueRedaction(h.storage, forEvent)
	if err != nil {
		log.Printf("[%s | %s] Non-fatal error trying to submit backwards-compatible redaction for event", forEvent.EventID(), forEvent.RoomID().String())
	}

	// Now onto the new stuff: find the community and their moderation bot's user ID, then send that user a to-device
	// message to instruct them to redact the event.
	room, err := h.storage.GetRoom(ctx, forEvent.RoomID().String())
	if err != nil {
		log.Printf("[%s | %s] Failed to get room information: %s", forEvent.EventID(), forEvent.RoomID().String(), err)
		return err
	}
	if room == nil {
		log.Printf("[%s | %s] No room found - skipping redaction", forEvent.EventID(), forEvent.RoomID().String())
		return nil // no room means no community either, which means no redaction
	}

	community, err := h.storage.GetCommunity(ctx, room.CommunityId)
	if err != nil {
		log.Printf("[%s | %s] Failed to get community information: %s", forEvent.EventID(), forEvent.RoomID().String(), err)
		return err
	}
	if community == nil {
		log.Printf("[%s | %s] No community found - skipping redaction", forEvent.EventID(), forEvent.RoomID().String())
		return nil // just like having no room, no community means no redaction
	}

	modbotUserId := internal.Dereference(community.Config.ModerationBotUserId)
	if modbotUserId == "" {
		log.Printf("[%s | %s | %s] No moderation bot user ID found - skipping redaction", forEvent.EventID(), forEvent.RoomID().String(), community.CommunityId)
		return nil // no moderation bot means no redaction
	}
	modbotUserIdParsed, err := spec.NewUserID(modbotUserId, true)
	if err != nil {
		log.Printf("[%s | %s | %s] Failed to validate moderation bot user ID: %s", forEvent.EventID(), forEvent.RoomID().String(), community.CommunityId, err)
		return err
	}

	// Create the body of the redaction command (we'll need to sign it so the moderation bot can verify it)
	body, err := json.Marshal(map[string]any{
		"command":  "redact",
		"room_id":  forEvent.RoomID().String(),
		"event_id": forEvent.EventID(),
	})
	if err != nil {
		log.Printf("[%s | %s | %s] Failed to marshal redaction command: %s", forEvent.EventID(), forEvent.RoomID().String(), community.CommunityId, err)
		return err // "should never happen"
	}
	signed, err := gomatrixserverlib.SignJSON(string(h.ServerName), PolicyServerKeyID, h.eventSigningKey, body)
	if err != nil {
		log.Printf("[%s | %s | %s] Failed to sign redaction command: %s", forEvent.EventID(), forEvent.RoomID().String(), community.CommunityId, err)
		return err // "should never happen"
	}

	// Create the to-device message EDU and send it
	toDeviceMsg := gomatrixserverlib.ToDeviceMessage{
		Sender:    h.localActor.String(),
		Type:      "org.matrix.policyserv.command",
		MessageID: storage.NextId(),
		Messages: map[string]map[string]json.RawMessage{
			modbotUserId: map[string]json.RawMessage{
				"*": signed,
			},
		},
	}
	msg, err := json.Marshal(toDeviceMsg)
	if err != nil {
		log.Printf("[%s | %s | %s] Failed to marshal to-device message: %s", forEvent.EventID(), forEvent.RoomID().String(), community.CommunityId, err)
		return err // "should never happen"
	}
	edu := &storage.StoredEdu{
		Destination: string(modbotUserIdParsed.Domain()),
		Payload: gomatrixserverlib.EDU{
			Type:    "m.direct_to_device",
			Content: msg,
		},
	}
	err = h.storage.InsertEdu(ctx, edu)
	if err != nil {
		log.Printf("[%s | %s | %s] Failed to insert to-device message EDU: %s", forEvent.EventID(), forEvent.RoomID().String(), community.CommunityId, err)
		return err
	}
	log.Printf("[%s | %s | %s] Queued redaction command to %s", forEvent.EventID(), forEvent.RoomID().String(), community.CommunityId, modbotUserId)

	// inserting will trigger a send (eventually), so we don't need to do that here

	return nil
}
