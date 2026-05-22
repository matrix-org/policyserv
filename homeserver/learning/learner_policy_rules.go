package learning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/storage"
)

type PolicyRulesLearner struct {
	storage storage.PersistentStorage
}

type minimalPolicyRule struct {
	Recommendation string `json:"recommendation,omitempty"`
	Entity         string `json:"entity,omitempty"`
}

func (p *PolicyRulesLearner) CanLearn(ctx context.Context, room *storage.StoredRoom, event gomatrixserverlib.PDU) (bool, error) {
	if event.Type() != "m.policy.rule.user" && event.Type() != "m.policy.rule.room" {
		return false, nil // wrong event type
	}
	if event.StateKey() == nil {
		return false, nil // not a state event
	}
	content := minimalPolicyRule{}
	err := json.Unmarshal(event.Content(), &content)
	if err != nil {
		return false, err
	}
	if content.Recommendation != "m.ban" {
		return false, nil // not a ban rule
	}
	if len(content.Entity) == 0 {
		return false, nil // no entity specified
	}
	return true, nil
}

func (p *PolicyRulesLearner) LearnFrom(ctx context.Context, room *storage.StoredRoom, roomState []gomatrixserverlib.PDU) error {
	entityBans := make(map[string]string)
	for _, pdu := range roomState {
		ok, err := p.CanLearn(ctx, room, pdu)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		content := minimalPolicyRule{}
		err = json.Unmarshal(pdu.Content(), &content)
		if err != nil {
			return errors.Join(fmt.Errorf("error parsing recommendation for %s / %s / %s", pdu.Type(), pdu.EventID(), pdu.RoomID()), err)
		}
		entityBans[content.Entity] = pdu.Type()
	}
	err := p.storage.SetListBanRules(ctx, room.RoomId, entityBans)
	if err != nil {
		return errors.Join(fmt.Errorf("error storing list ban rules for %s", room.RoomId), err)
	}
	return nil
}
