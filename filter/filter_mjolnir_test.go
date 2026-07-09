package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestMjolnirFilter(t *testing.T) {
	ctx := context.Background()

	mjolnirRoomId := "!mjolnir:example.org"
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			MjolnirFilterEnabled: internal.Pointer(true),
		},
		InstanceConfig: &config.InstanceConfig{
			MjolnirFilterRoomID: mjolnirRoomId,
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{MjolnirFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything is neutral by default in the test
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	err = memStorage.SetListBanRules(ctx, mjolnirRoomId, map[string]string{
		"@alice*:example.org": "m.policy.rule.user",
		"*.example.org":       "m.policy.rule.server",
	})
	assert.NoError(t, err)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@aliceeeeeeeeee:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@bob:subdomain.example.org",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@alic:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	neutralEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@bob:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	noopEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.wrong_event_type_for_filter",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})

	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.OtherGeneral))
	AssertCheckEvent(t, set, spammyEvent2, harms.ProhibitedContent(harms.OtherGeneral))
	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
	AssertCheckEvent(t, set, neutralEvent2, harms.NeutralContent())
	AssertCheckEvent(t, set, noopEvent1, harms.NeutralContent())
}
