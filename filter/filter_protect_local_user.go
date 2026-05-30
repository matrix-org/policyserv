package filter

import (
	"context"
	"fmt"
	"log"

	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/matrix-org/policyserv/filter/classification"
)

// Developer note: this filter is force-enabled and cannot be disabled.

const ProtectLocalUserFilterName = "ProtectLocalUserFilter"

func init() {
	mustRegister(ProtectLocalUserFilterName, &ProtectLocalUserFilter{})
}

type ProtectLocalUserFilter struct {
}

func (f *ProtectLocalUserFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedProtectLocalUserFilter{
		set:         set,
		localUserId: spec.NewUserIDOrPanic(fmt.Sprintf("@%s:%s", set.instanceConfig.JoinLocalpart, set.instanceConfig.HomeserverName), false),
	}, nil
}

type InstancedProtectLocalUserFilter struct {
	set         *Set
	localUserId spec.UserID
}

func (p *InstancedProtectLocalUserFilter) Name() string {
	return ProtectLocalUserFilterName
}

func (p *InstancedProtectLocalUserFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	if input.Event.Type() == "m.room.member" {
		// If the state key is for our user ID, but the sender is different, then someone is trying to invite, kick, or ban our user. We
		// don't want them to do that because it can break things - we should be managing it ourselves. The state key and sender will be
		// the same on joins and self-leaves.
		// XXX: This also denies invites, but we don't support them yet anyway - https://github.com/matrix-org/policyserv/issues/91
		if input.Event.StateKeyEquals(p.localUserId.String()) && input.Event.SenderID().ToUserID().String() != p.localUserId.String() {
			log.Printf("[%s | %s] %s tried to add/remove policyserv user improperly", input.Event.EventID(), input.Event.RoomID(), input.Event.SenderID())
			return []classification.Classification{classification.Spam}, nil
		}
	}
	return nil, nil
}
