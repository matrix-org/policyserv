package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
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
			EnabledNames:           []string{UntrustedMediaFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
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

	assertSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		vecs, err := set.CheckEvent(context.Background(), event, nil)
		assert.NoError(t, err)
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
		} else {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
		}
	}
	assertSpamVector(spammyEvent1, true)
	assertSpamVector(spammyEvent2, true)
	assertSpamVector(neutralEvent1, false)
	assertSpamVector(neutralEvent2, false)
	assertSpamVector(neutralEvent3, false)
	assertSpamVector(neutralEvent4, false)
	assertSpamVector(noopEvent1, false)
}
