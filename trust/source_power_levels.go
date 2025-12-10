package trust

import (
	"context"
	"database/sql"
	"errors"

	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/gomatrixserverlib"
)

// PowerLevelsSource - uses the room's power levels to determine trust levels. Above-default power levels are trusted.
type PowerLevelsSource struct {
	db storage.PersistentStorage
}

func NewPowerLevelsSource(db storage.PersistentStorage) (*PowerLevelsSource, error) {
	return &PowerLevelsSource{
		db: db,
	}, nil
}

func (s *PowerLevelsSource) HasCapability(ctx context.Context, userId string, roomId string, capability Capability) (Tristate, error) {
	hasPl, err := s.IsUserAboveDefault(ctx, roomId, userId)
	if err != nil {
		return TristateDefault, err
	}

	if hasPl {
		return TristateTrue, nil
	}

	return TristateDefault, nil
}

// Dev note: below here we hide the persistence details from the rest of the code for maintenance purposes. Please keep
// this stuff together for visibility/ease of maintenance.

const powerLevelsSourceName = "power_levels"

func (s *PowerLevelsSource) IsUserAboveDefault(ctx context.Context, roomId string, userId string) (bool, error) {
	val := &gomatrixserverlib.PowerLevelContent{}
	err := s.db.GetTrustData(ctx, powerLevelsSourceName, roomId, &val)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	userPl, ok := val.Users[userId]
	if !ok {
		userPl = val.UsersDefault
	}

	// Currently, all high power level users have all capabilities.
	return userPl >= val.StateDefault, nil
}

func (s *PowerLevelsSource) ImportData(ctx context.Context, roomId string, powerLevelsEvent gomatrixserverlib.PDU) error {
	if powerLevelsEvent.Type() != "m.room.power_levels" || powerLevelsEvent.StateKey() == nil || *(powerLevelsEvent.StateKey()) != "" {
		return errors.New("not a power levels event")
	}

	data, err := gomatrixserverlib.NewPowerLevelContentFromEvent(powerLevelsEvent)
	if err != nil {
		return err
	}

	return s.db.SetTrustData(ctx, powerLevelsSourceName, roomId, data)
}
