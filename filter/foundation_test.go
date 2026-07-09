package filter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestMatrixFoundationIntents(t *testing.T) {
	//t.Parallel()

	// Note: this test doesn't need to use real config values or events from production. Its purpose is to
	// ensure that it's *possible* to separate spam from neutral events when layering multiple filters.

	mjolnirRoomId := "!mjolnir:example.org"

	communityConfig, err := config.NewDefaultCommunityConfig()
	assert.NoError(t, err)
	communityConfig.SenderPrefilterAllowedSenders = &[]string{foundationAdmin}
	communityConfig.MjolnirFilterEnabled = internal.Pointer(true)
	communityConfig.DensityFilterMaxDensity = internal.Pointer(0.95)
	communityConfig.HellbanPostfilterMinutes = internal.Pointer(60) // large number to ensure the hellban will never expire in the test

	cnf := &SetConfig{
		CommunityId: "foundation",
		InstanceConfig: &config.InstanceConfig{
			MjolnirFilterRoomID: mjolnirRoomId,
		},
		CommunityConfig: communityConfig,
		Groups: []*SetGroupConfig{{
			// Prefilters
			EnabledNames:          []string{HellbanPrefilterName, SenderFilterName, EventTypeFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything starts as neutral by default in the test
		}, {
			// Normal filters
			EnabledNames:          []string{DensityFilterName, LengthFilterName, ManyAtsFilterName, MediaFilterName, MentionsFilterName, MjolnirFilterName, TrimLengthFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // only run on still-neutral events
		}, {
			// Postfilters
			EnabledNames:          []string{HellbanPostfilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassProhibited}, // only run hellban on spammy events
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)
	assert.NoError(t, memStorage.SetListBanRules(context.Background(), mjolnirRoomId, map[string]string{
		"@banned*:example.org": "m.policy.rule.user",
	}))
	assert.NoError(t, memStorage.SetUserIdsAndDisplayNamesByRoomId(context.Background(), foundationRoomId, foundationMentionIds, foundationMentionNames))

	for i, run := range foundationEvents {
		t.Run(fmt.Sprintf("%d (%s)", i, run.Event.EventID()), func(t *testing.T) {
			AssertCheckEvent(t, set, run.Event, run.Expected)

			// Give things some time to settle before moving on to the next test case. This is primarily important for the
			// hellban test cases to ensure the cause event creates a hellban before the effect event.
			time.Sleep(100 * time.Millisecond)
		})
	}

	// We deferred a bunch of close operations, but sometimes these operations can race with the test cases above causing
	// a hellban. So, give things a moment to settle before closing.
	time.Sleep(250 * time.Millisecond)
}

type foundationTestCase struct {
	Event    gomatrixserverlib.PDU
	Expected *harms.ContentInfo
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
			Expected: harms.AllowedContent(),
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
			Expected: harms.NeutralContent(),
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
			Expected: harms.ProhibitedContent(harms.OtherGeneral, harms.SpamFlooding),
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
			Expected: harms.ProhibitedContent(harms.SpamGeneral, harms.SpamFlooding, harms.PolicyservMedia),
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
			Expected: harms.NeutralContent(),
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
			Expected: harms.ProhibitedContent(harms.SpamFlooding),
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
			Expected: harms.ProhibitedContent(harms.SpamFlooding),
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
			Expected: harms.ProhibitedContent(harms.SpamFlooding),
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
			Expected: harms.ProhibitedContent(harms.SpamFlooding),
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
			Expected: harms.ProhibitedContent(harms.SpamFlooding),
		},
	}
}
