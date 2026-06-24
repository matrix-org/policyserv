package filter

import (
	"context"

	"github.com/matrix-org/policyserv/harms"
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

func (f *InstancedMjolnirFilter) CheckEvent(ctx context.Context, input *EventInput) (*harms.ContentInfo, error) {
	// Return early on non-message events
	if input.Event.Type() != "m.room.message" {
		return harms.NeutralContent(), nil
	}

	banned, err := f.set.storage.IsUserBannedInList(ctx, f.policyRoomId, string(input.Event.SenderID()))
	if err != nil {
		return nil, err
	}

	if banned {
		return harms.ProhibitedContent(harms.OtherGeneral), nil
	}

	return harms.NeutralContent(), nil
}
