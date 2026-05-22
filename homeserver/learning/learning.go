package learning

import (
	"context"
	"errors"
	"log"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/trust"
)

// RoomStateLearner is an EventStateLearner which calls into other EventStateLearner instances
type RoomStateLearner struct {
	storage  storage.PersistentStorage
	learners []EventStateLearner
}

func NewRoomStateLearner(storage storage.PersistentStorage) (EventStateLearner, error) {
	// XXX: It's a little bad that we're hardcoding learners this way. Ideally we'd use a registry.
	mustConstruct := func(learner EventStateLearner, err error) EventStateLearner {
		if err != nil {
			log.Fatal("failed to construct learner", err)
		}
		return learner
	}
	learners := []EventStateLearner{
		&RoomMembersLearner{storage: storage},
		&PolicyRulesLearner{storage: storage},
		mustConstruct(trust.NewPowerLevelsSource(storage)),
		mustConstruct(trust.NewCreatorSource(storage)),
	}

	return &RoomStateLearner{
		storage:  storage,
		learners: learners,
	}, nil
}

func (l *RoomStateLearner) CanLearn(ctx context.Context, room *storage.StoredRoom, event gomatrixserverlib.PDU) (bool, error) {
	for _, learner := range l.learners {
		canLearn, err := learner.CanLearn(ctx, room, event)
		if err != nil {
			return false, err
		}
		if canLearn {
			return true, nil
		}
	}
	return false, nil
}

func (l *RoomStateLearner) LearnFrom(ctx context.Context, room *storage.StoredRoom, roomState []gomatrixserverlib.PDU) error {
	errs := make([]error, 0)
	for _, learner := range l.learners {
		err := learner.LearnFrom(ctx, room, roomState)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
