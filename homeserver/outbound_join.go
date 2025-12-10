package homeserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/gomatrixserverlib/fclient"
	"github.com/matrix-org/gomatrixserverlib/spec"
)

type internalJoinClient struct {
	upstream fclient.FederationClient
}

func (c *internalJoinClient) MakeJoin(ctx context.Context, origin spec.ServerName, via spec.ServerName, roomId string, userId string) (gomatrixserverlib.MakeJoinResponse, error) {
	res, err := c.upstream.MakeJoin(ctx, origin, via, roomId, userId)
	return &res, err
}

func (c *internalJoinClient) SendJoin(ctx context.Context, origin spec.ServerName, via spec.ServerName, event gomatrixserverlib.PDU) (gomatrixserverlib.SendJoinResponse, error) {
	res, err := c.upstream.SendJoin(ctx, origin, via, event)
	return &res, err
}

func (h *Homeserver) JoinRoom(ctx context.Context, roomId string, via string, communityId string) (*storage.StoredRoom, error) {
	parsedRoomId, err := spec.NewRoomID(roomId)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("error parsing room ID %s", roomId), err)
	}

	// First, are we already joined?
	room, err := h.storage.GetRoom(ctx, roomId)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("error looking up room %s", roomId), err)
	}
	if room != nil {
		log.Printf("Already joined room %s", roomId)
		return room, nil
	}

	join, fedErr := gomatrixserverlib.PerformJoin(ctx, &internalJoinClient{upstream: h.client}, gomatrixserverlib.PerformJoinInput{
		UserID:     &h.localActor,
		RoomID:     parsedRoomId,
		ServerName: spec.ServerName(via),
		Content: map[string]interface{}{
			"membership": "join",
		},
		Unsigned:   make(map[string]interface{}),
		PrivateKey: h.signingKey,
		KeyID:      h.KeyId,
		KeyRing:    h.keyRing,
		EventProvider: func(roomVersion gomatrixserverlib.RoomVersion, eventIds []string) ([]gomatrixserverlib.PDU, error) {
			pdus := make([]gomatrixserverlib.PDU, 0)
			for _, eventId := range eventIds {
				txn, err := h.client.GetEvent(ctx, h.ServerName, spec.ServerName(via), eventId)
				if err != nil {
					return nil, err
				}

				if len(txn.PDUs) != 1 {
					return nil, fmt.Errorf("expected 1 PDU, got %d", len(txn.PDUs))
				}

				ver := gomatrixserverlib.MustGetRoomVersion(roomVersion)
				pdu, err := ver.NewEventFromUntrustedJSON(txn.PDUs[0])
				if err != nil {
					return nil, err
				}

				pdus = append(pdus, pdu)
			}
			return pdus, nil
		},
		UserIDQuerier: func(roomId spec.RoomID, senderId spec.SenderID) (*spec.UserID, error) {
			return senderId.ToUserID(), nil
		},
	})
	if fedErr != nil && fedErr.Err != nil {
		return nil, errors.Join(fmt.Errorf("error joining room %s", roomId), fedErr)
	}
	if join == nil {
		return nil, errors.New("join response was nil")
	}

	room = &storage.StoredRoom{
		RoomId:                         join.JoinEvent.RoomID().String(),
		RoomVersion:                    string(join.JoinEvent.Version()),
		ModeratorUserId:                "",
		LastCachedStateTimestampMillis: 0,
		CommunityId:                    communityId,
	}
	err = h.storage.UpsertRoom(ctx, room)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("error upserting room %s", join.JoinEvent.RoomID()), err)
	}

	log.Printf("Joined %s (v:%s)", join.JoinEvent.RoomID().String(), join.JoinEvent.Version())
	return room, nil
}

// JoinRooms - Attempts to join rooms in order, retrying a number of times if required. This is a
// non-idempotent function: it will store successful joins until an error occurs.
func (h *Homeserver) JoinRooms(ctx context.Context, roomIds []string, via string, communityId string) error {
	for _, roomId := range roomIds {
		errCount := 0
		for {
			if _, err := h.JoinRoom(ctx, roomId, via, communityId); err != nil {
				if errCount > 5 {
					return errors.Join(fmt.Errorf("error joining room %s through %s", roomId, via), err)
				}
				errCount++
				log.Printf("Error %d joining room %s through %s: %v", errCount, roomId, via, err)
				time.Sleep(time.Second)
			} else {
				break
			}
		}
	}
	return nil
}
