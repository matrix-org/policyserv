package filter

import (
	"context"
	"log"
	"strconv"
	"strings"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/test"
	"github.com/matrix-org/gomatrixserverlib"
	"github.com/stretchr/testify/assert"
)

func TestMatrixFoundationIntents(t *testing.T) {
	//t.Parallel()

	// Note: this test doesn't need to use real config values or events from production. Its purpose is to
	// ensure that it's *possible* to separate spam from neutral events when layering multiple filters.

	spamThreshold := 0.8
	mjolnirRoomId := "!mjolnir:example.org"

	communityConfig, err := config.NewDefaultCommunityConfig()
	assert.NoError(t, err)
	communityConfig.SpamThreshold = spamThreshold
	communityConfig.SenderPrefilterAllowedSenders = []string{foundationAdmin}
	communityConfig.MjolnirFilterEnabled = true
	communityConfig.DensityFilterMaxDensity = 0.95
	communityConfig.HellbanPostfilterMinutes = 5

	cnf := &SetConfig{
		CommunityId: "foundation",
		InstanceConfig: &config.InstanceConfig{
			MjolnirFilterRoomID: mjolnirRoomId,
		},
		CommunityConfig: communityConfig,
		Groups: []*SetGroupConfig{{
			// Prefilters
			EnabledNames:           []string{HellbanPrefilterName, SenderFilterName, EventTypeFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}, {
			// Normal filters
			EnabledNames:           []string{DensityFilterName, LengthFilterName, ManyAtsFilterName, MediaFilterName, MentionsFilterName, MjolnirFilterName, TrimLengthFilterName},
			MinimumSpamVectorValue: 0.2,
			MaximumSpamVectorValue: 0.7,
		}, {
			// Postfilters
			EnabledNames:           []string{HellbanPostfilterName},
			MinimumSpamVectorValue: spamThreshold,
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
	assert.NoError(t, memStorage.SetListBanRules(context.Background(), mjolnirRoomId, map[string]string{
		"@banned*:example.org": "m.policy.rule.user",
	}))
	assert.NoError(t, memStorage.SetUserIdsAndDisplayNamesByRoomId(context.Background(), foundationRoomId, foundationMentionIds, foundationMentionNames))

	for i, run := range foundationEvents {
		log.Printf("Running test case %d - %s", i, run.Event.EventID())
		vecs, err := set.CheckEvent(context.Background(), run.Event, nil)
		assert.NoError(t, err)
		assert.NotNil(t, vecs)
		if run.IsSpam {
			assert.LessOrEqualf(t, spamThreshold, vecs.GetVector(classification.Spam), "%d (%s) should be spam, but was %f", i, run.Event.EventID(), vecs.GetVector(classification.Spam))
		} else {
			assert.Greaterf(t, spamThreshold, vecs.GetVector(classification.Spam), "%d (%s) should NOT be spam, but was %f", i, run.Event.EventID(), vecs.GetVector(classification.Spam))
		}
	}
}

type foundationTestCase struct {
	Event  gomatrixserverlib.PDU
	IsSpam bool
}

const foundationRoomId = "!foo:example.org"
const foundationAdmin = "@admin:example.org"

var foundationMentionIds = make([]string, 0, 30)
var foundationMentionNames = make([]string, 0, 30)
var foundationEvents []foundationTestCase

func init() {
	for i := 0; i < 30; i++ {
		foundationMentionIds = append(foundationMentionIds, "@mention"+strconv.Itoa(i)+":example.org")
		foundationMentionNames = append(foundationMentionNames, "Mention"+strconv.Itoa(i))
	}

	foundationEvents = []foundationTestCase{
		{
			Event: test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$admin1",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  foundationAdmin, // Admins should always be allowed to send messages
				Content: map[string]any{
					"body":    "doesn't matter",
					"msgtype": "m.image", // try to activate the filter rules
				},
			}),
			IsSpam: false,
		}, {
			Event: test.MustMakePDU(&test.BaseClientEvent{
				// This event is completely neutral: sender isn't special, event type is a message, and the body is fine (but long, to try triggering a few filters)
				EventId: "$user1",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  "@regular_user:example.org",
				Content: map[string]any{
					"msgtype": "m.text",
					"body":    "hello world! I'm trying to get help with running policyserv. Can someone help me understand what these logs mean?\n```text\n2025/09/09 15:42:05 Registered DensityFilter as &filter.DensityFilter{}\n2025/09/09 15:42:05 Registered EventTypeFilter as &filter.EventTypeFilter{}\n2025/09/09 15:42:05 Registered HellbanPrefilter as &filter.HellbanPrefilter{}\n2025/09/09 15:42:05 Registered HellbanPostfilter as &filter.HellbanPostfilter{}\n2025/09/09 15:42:05 Registered KeywordFilter as &filter.KeywordFilter{}\n2025/09/09 15:42:05 Registered LengthFilter as &filter.LengthFilter{}\n2025/09/09 15:42:05 Registered ManyAtsFilter as &filter.ManyAtsFilter{}\n2025/09/09 15:42:05 Registered MediaFilter as &filter.MediaFilter{}\n2025/09/09 15:42:05 Registered MentionsFilter as &filter.MentionsFilter{}\n2025/09/09 15:42:05 Registered MjolnirFilter as &filter.MjolnirFilter{}\n2025/09/09 15:42:05 Registered SenderFilter as &filter.SenderFilter{}\n2025/09/09 15:42:05 Registered TrimLengthFilter as &filter.TrimLengthFilter{}\n2025/09/09 15:42:05 Registered FixedFilter as &filter.FixedCanBeInstancedFilter{}\n2025/09/09 15:42:05 Registered ErrorFilter as &filter.ErrorFilter{}```\nThanks!",
				},
			}),
			IsSpam: false,
		}, {
			Event: test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$banned_sender1",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  "@banned_sender:example.org", // actual mjolnir rule is "@banned*:example.org"
				Content: map[string]any{
					"body": "doesn't matter",
				},
			}),
			IsSpam: true,
		}, {
			Event: test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$media1",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  "@user_media2:example.org",
				Content: map[string]any{
					"msgtype": "m.image", // should trigger media filter
					"body":    "hello world",
				},
			}),
			IsSpam: true,
		}, {
			Event: test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$mentions_neutral1",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  "@user_mentions_neutral1:example.org",
				Content: map[string]any{
					"msgtype": "m.text",
					"body":    strings.Join(foundationMentionNames[0:5], " "), // not enough to trigger filter
				},
			}),
			IsSpam: false,
		}, {
			Event: test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$mentions_spam1",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  "@user_mentions_1:example.org",
				Content: map[string]any{
					"msgtype": "m.text",
					"body":    strings.Join(foundationMentionNames, " "), // should be enough to trigger filter
				},
			}),
			IsSpam: true,
		}, {
			Event: test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$length_spam1",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  "@user_length_1:example.org",
				Content: map[string]any{
					"msgtype": "m.text",
					"body":    strings.Repeat("spammy spam", 10000),
				},
			}),
			IsSpam: true,
		}, {
			Event: test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$density_spam1",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  "@user_density_1:example.org",
				Content: map[string]any{
					"msgtype": "m.text",
					"body":    strings.Repeat("spammy-spam", 100),
				},
			}),
			IsSpam: true,
		}, {
			Event: test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$hellban_cause",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  "@hellban:example.org",
				Content: map[string]any{
					"msgtype": "m.text",
					"body":    strings.Repeat("spammy-spam", 100), // anything to cause a ban
				},
			}),
			IsSpam: true,
		}, {
			Event: test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$hellban_effect",
				RoomId:  foundationRoomId,
				Type:    "m.room.message",
				Sender:  "@hellban:example.org",
				Content: map[string]any{
					"msgtype": "m.text",
					"body":    "fine, but hellbanned", // a normal message, but the user should be picked up by hellban
				},
			}),
			IsSpam: true,
		},
	}
}
