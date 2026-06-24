package filter

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

type members struct {
	UserIds      []string `json:"user_ids"`
	DisplayNames []string `json:"display_names"`
}

type mentionsTestCase struct {
	TestName      string
	EventType     string
	Body          string
	FormattedBody string
	Mentions      []string
	Members       *members
	WantNeutral   bool
	MentionsCount int
}

func TestMentionsFilter(t *testing.T) {
	ctx := context.Background()

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			MentionFilterMaxMentions:        internal.Pointer(3),
			MentionFilterMinPlaintextLength: internal.Pointer(5),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{MentionsFilterName},
			RunOnClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything is neutral by default in the test
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	testCases := []mentionsTestCase{
		{
			EventType: "m.room.message",
			TestName:  "basic single mention is not spam",
			Body:      "Hi bob",
			Members: &members{
				UserIds:      []string{"@b:example.com"},
				DisplayNames: []string{"bob"},
			},
			WantNeutral:   true,
			MentionsCount: 1,
		},
		{
			EventType:     "m.room.message",
			TestName:      "basic single user ID mention is not spam",
			Body:          "Hi @b:example.com",
			FormattedBody: "Hi @b:example.com",
			Mentions:      []string{"@b:example.com"},
			Members: &members{
				UserIds:      []string{"@b:example.com"},
				DisplayNames: []string{"bob"},
			},
			WantNeutral:   true,
			MentionsCount: 2,
		},
		{
			EventType: "m.room.message",
			TestName:  "many mentions is spam",
			Body:      "Hi alice bob charlie",
			Members: &members{
				UserIds:      []string{"@a:example.com", "@b:example.com", "@c:example.com"},
				DisplayNames: []string{"alice", "bob", "charlie"},
			},
			WantNeutral:   false,
			MentionsCount: 5,
		},
		{
			EventType:     "m.room.message",
			TestName:      "many mentions across multiple fields is spam",
			Body:          "Hi alice",
			FormattedBody: "Hi bob",
			Mentions:      []string{"@c:example.com"},
			Members: &members{
				UserIds:      []string{"@a:example.com", "@b:example.com", "@c:example.com"},
				DisplayNames: []string{"alice", "bob", "charlie"},
			},
			WantNeutral:   false,
			MentionsCount: 4,
		},
		{
			EventType: "m.room.message",
			TestName:  "mentions for user ID is spam",
			Body:      "Hi @a:example.com @b:example.com @c:example.com ",
			Members: &members{
				UserIds:      []string{"@a:example.com", "@b:example.com", "@c:example.com"},
				DisplayNames: []string{"alice", "bob", "charlie"},
			},
			WantNeutral:   false,
			MentionsCount: 6,
		},
		{
			EventType: "org.example.wrong_event_type_for_filter",
			TestName:  "no-ops on non-message events",
			Body:      "Hi bob",
			Members: &members{
				UserIds:      []string{"@b:example.com"},
				DisplayNames: []string{"bob"},
			},
			WantNeutral:   true,
			MentionsCount: 0,
		},
		{
			EventType: "m.room.message",
			TestName:  "many low-character names are not spam",
			Body:      "a b c d e f g h i j k l m n o p q r s t u v w x y z",
			Members: &members{
				UserIds:      []string{"@still_a_mention:example.org"},
				DisplayNames: strings.Split("a b c d e f g h i j k l m n o p q r s t u v w x y z", " "),
			},
			WantNeutral:   true,
			MentionsCount: 1,
		},
		{
			EventType: "m.room.message",
			TestName:  "detects spam for names for long enough names",
			Body:      "abcdef ghijkl mnopqr stuvwx yz",
			Members: &members{
				UserIds:      []string{"@still_a_mention:example.org"},
				DisplayNames: []string{"abcdef", "ghijkl", "mnopqr", "stuvwx", "yz"},
			},
			WantNeutral:   false,
			MentionsCount: 5,
		},
	}

	roomId := "!foo:example.org"
	for _, tc := range testCases {
		t.Run(tc.TestName, func(t *testing.T) {
			event := test.MustMakePDU(&test.BaseClientEvent{
				EventId: "$testcase",
				RoomId:  roomId,
				Type:    tc.EventType,
				Content: map[string]any{
					"m.mentions": map[string]any{
						"user_ids": tc.Members.UserIds,
					},
					"body":           tc.Body,
					"formatted_body": tc.FormattedBody,
				},
			})
			err = memStorage.SetUserIdsAndDisplayNamesByRoomId(ctx, roomId, tc.Members.UserIds, tc.Members.DisplayNames)
			assert.NoError(t, err)

			mentionsFilter, ok := set.groups[0].filters[0].(*InstancedMentionsFilter)
			assert.True(t, ok)
			numMentions, err := mentionsFilter.CountMentionsToLimit(context.Background(), event, math.MaxInt)
			assert.NoError(t, err)
			assert.Equal(t, tc.MentionsCount, numMentions)

			if tc.WantNeutral {
				AssertCheckEvent(t, set, event, harms.NeutralContent())
			} else {
				AssertCheckEvent(t, set, event, harms.ProhibitedContent(harms.SpamFlooding))
			}
		})
	}
}
