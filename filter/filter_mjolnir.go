package filter

import (
	"context"

	"github.com/matrix-org/policyserv/filter/classification"
)

const MjolnirFilterName = "MjolnirFilter"

func init() {
	mustRegister(MjolnirFilterName, &MjolnirFilter{})
}

type MjolnirFilter struct {
}

func (m *MjolnirFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedMjolnirFilter{
		set:          set,
		policyRoomId: set.instanceConfig.MjolnirFilterRoomID,
	}, nil
}

type InstancedMjolnirFilter struct {
	set          *Set
	policyRoomId string
}

func (f *InstancedMjolnirFilter) Name() string {
	return MjolnirFilterName
}

func (f *InstancedMjolnirFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	// Return early on non-message events
	if input.Event.Type() != "m.room.message" {
		return nil, nil
	}

	banned, err := f.set.storage.IsUserBannedInList(ctx, f.policyRoomId, string(input.Event.SenderID()))
	if err != nil {
		return nil, err
	}

	if banned {
		return []classification.Classification{classification.Spam}, nil
	}

	return nil, nil
}
