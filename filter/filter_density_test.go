package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestDensityFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			DensityFilterMaxDensity:       internal.Pointer(0.5),
			DensityFilterMinTriggerLength: internal.Pointer(10),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{DensityFilterName},
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

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "aaaaaaaaaaaaaaaaaaaaaaaa", // ~1.0000 density
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "a b c d e f", // ~0.5454 density
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "a          b", // ~0.1666 density
		},
	})
	neutralEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "  a  b  c  d  e  f  g  h  ", // ~0.3076 density
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
	noopEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "2short",
		},
	})

	assertSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		vecs, err := set.CheckEvent(context.Background(), event, nil)
		assert.NoError(t, err)
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
			assert.Equal(t, 1.0, vecs.GetVector(classification.Volumetric))
		} else {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
			assert.Equal(t, 0.0, vecs.GetVector(classification.Volumetric))
		}
	}
	assertSpamVector(spammyEvent1, true)
	assertSpamVector(spammyEvent2, true)
	assertSpamVector(neutralEvent1, false)
	assertSpamVector(neutralEvent2, false)
	assertSpamVector(noopEvent1, false)
	assertSpamVector(noopEvent2, false)

	// Also test the text filter implementation
	assertTextSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		body := gjson.Get(string(event.Content()), "body").String()
		vecs, err := set.CheckText(context.Background(), body)
		assert.NoError(t, err)
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
			assert.Equal(t, 1.0, vecs.GetVector(classification.Volumetric))
		} else {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
			assert.Equal(t, 0.0, vecs.GetVector(classification.Volumetric))
		}
	}
	assertTextSpamVector(spammyEvent1, true)
	assertTextSpamVector(spammyEvent2, true)
	assertTextSpamVector(neutralEvent1, false)
	assertTextSpamVector(neutralEvent2, false)
	//assertTextSpamVector(noopEvent1, false) // text doesn't have a concept of event types, so skip this one
	assertTextSpamVector(noopEvent2, false)
}
