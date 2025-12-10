package trust

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"slices"

	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/gomatrixserverlib"
)

// CreatorSource - trusts v12+ room creators as high power level users
type CreatorSource struct {
	db storage.PersistentStorage
}

func NewCreatorSource(db storage.PersistentStorage) (*CreatorSource, error) {
	return &CreatorSource{
		db: db,
	}, nil
}

func (s *CreatorSource) HasCapability(ctx context.Context, userId string, roomId string, capability Capability) (Tristate, error) {
	creators, err := s.GetCreators(ctx, roomId)
	if err != nil {
		return TristateDefault, err
	}

	// Currently, all creators have all capabilities.
	if slices.Contains(creators, userId) {
		return TristateTrue, nil
	}

	return TristateDefault, nil
}

// Dev note: below here we hide the persistence details from the rest of the code for maintenance purposes. Please keep
// this stuff together for visibility/ease of maintenance.

const creatorSourceName = "creator"

type creatorData struct {
	CreatorUserIds []string `json:"creator_user_ids"`
}

func (s *CreatorSource) GetCreators(ctx context.Context, roomId string) ([]string, error) {
	val := &creatorData{}
	err := s.db.GetTrustData(ctx, creatorSourceName, roomId, &val)
	if errors.Is(err, sql.ErrNoRows) {
		return make([]string, 0), nil
	} else if err != nil {
		return nil, err
	}
	return val.CreatorUserIds, nil
}

func (s *CreatorSource) ImportData(ctx context.Context, roomId string, createEvent gomatrixserverlib.PDU) error {
	if createEvent.Type() != "m.room.create" || createEvent.StateKey() == nil || *(createEvent.StateKey()) != "" {
		return errors.New("not a create event")
	}

	content := struct {
		RoomVersion        string   `json:"room_version"`
		AdditionalCreators []string `json:"additional_creators"`
	}{}
	err := json.Unmarshal(createEvent.Content(), &content)
	if err != nil {
		return err
	}

	roomVersion, err := gomatrixserverlib.GetRoomVersion(gomatrixserverlib.RoomVersion(content.RoomVersion))
	if err != nil {
		return err
	}

	// Skip persisting creators for non-v12+ rooms because they don't give creator permissions
	if !roomVersion.PrivilegedCreators() {
		log.Printf("Skipping creator trust data for room %s because it's not v12+", roomId)
		return nil
	}

	// Sanity check, because sometimes this goes wrong
	if !createEvent.SenderID().IsUserID() {
		return errors.New("sender is not a user ID for unknown reason")
	}

	data := &creatorData{
		CreatorUserIds: make([]string, 0),
	}
	data.CreatorUserIds = append(data.CreatorUserIds, createEvent.SenderID().ToUserID().String())
	data.CreatorUserIds = append(data.CreatorUserIds, content.AdditionalCreators...)

	return s.db.SetTrustData(ctx, creatorSourceName, roomId, data)
}
