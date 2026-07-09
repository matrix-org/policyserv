package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/matrix-org/policyserv/trust"
	"github.com/stretchr/testify/assert"
)

func TestUntrustedMediaFilter(t *testing.T) {
	t.Parallel()

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			UntrustedMediaFilterMediaTypes:     &[]string{"m.sticker", "m.image"},
			UntrustedMediaFilterUsePowerLevels: internal.Pointer(true),
			UntrustedMediaFilterUseMuninn:      internal.Pointer(true),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{UntrustedMediaFilterName},
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

	// Populate trust sources with actual data
	muninnSource, err := trust.NewMuninnHallSource(memStorage)
	assert.NoError(t, err)
	assert.NotNil(t, muninnSource)
	err = muninnSource.ImportData(context.Background(), trust.MuninnHallMemberDirectory{
		"muninn.example.org": {"@admin:example.org"},
	})
	assert.NoError(t, err)
	plSource, err := trust.NewPowerLevelsSource(memStorage)
	assert.NoError(t, err)
	assert.NotNil(t, plSource)
	stateKey := ""
	err = plSource.ImportData(context.Background(), "!foo:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "m.room.power_levels",
		StateKey: &stateKey,
		Sender:   "@powerlevels:example.org",
		Content: map[string]any{
			"state_default": 50,
			"users_default": 0,
			"users": map[string]any{
				"@user:powerlevels.example.org": 100,
			},
		},
	}))
	assert.NoError(t, err)
	creatorSource, err := trust.NewCreatorSource(memStorage)
	assert.NoError(t, err)
	assert.NotNil(t, creatorSource)
	err = creatorSource.ImportData(context.Background(), "!foo:example.org", test.MustMakePDU(&test.BaseClientEvent{
		Type:     "m.room.create",
		StateKey: &stateKey,
		Sender:   "@user:creator.example.org",
		Content: map[string]any{
			"room_version":        "12",
			"additional_creators": []string{"@user:additionalcreator.example.org"},
		},
	}))

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Sender:  "@spam:example.org",
		Type:    "m.sticker",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Sender:  "@spam2:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.image",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Sender:  "@user:muninn.example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.image",
		},
	})
	neutralEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral2",
		RoomId:  "!foo:example.org",
		Sender:  "@user:powerlevels.example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.image",
		},
	})
	neutralEvent3 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral3",
		RoomId:  "!foo:example.org",
		Sender:  "@user:creator.example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.image",
		},
	})
	neutralEvent4 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral4",
		RoomId:  "!foo:example.org",
		Sender:  "@user:additionalcreator.example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"msgtype": "m.image",
		},
	})
	noopEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop1",
		RoomId:  "!foo:example.org",
		Sender:  "@anyone:example.org",
		Type:    "org.example.wrong_event_type_for_filter",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})

	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamGeneral, harms.PolicyservMedia))
	AssertCheckEvent(t, set, spammyEvent2, harms.ProhibitedContent(harms.SpamGeneral, harms.PolicyservMedia))
	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
	AssertCheckEvent(t, set, neutralEvent2, harms.NeutralContent())
	AssertCheckEvent(t, set, neutralEvent3, harms.NeutralContent())
	AssertCheckEvent(t, set, neutralEvent4, harms.NeutralContent())
	AssertCheckEvent(t, set, noopEvent1, harms.NeutralContent())
}
