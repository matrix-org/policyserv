package learning

import (
	"context"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/storage"
)

// EventStateLearner is something which is capable of "learning" specific parts of room state. Usually these
// will convert the state events received into a more usable format for later use.
type EventStateLearner interface {
	// CanLearn returns true if this can learn anything from the given event. Usually this is a type check.
	// Note that the given event might not be a state event.
	CanLearn(ctx context.Context, room *storage.StoredRoom, event gomatrixserverlib.PDU) (bool, error)

	// LearnFrom processes the entire room state for a given room and learns whatever it can from it. The room
	// state is not filtered by CanLearn - the implementation can do so if needed. The room state will always
	// contain state events, and all events will be for the same room.
	LearnFrom(ctx context.Context, room *storage.StoredRoom, roomState []gomatrixserverlib.PDU) error
}
