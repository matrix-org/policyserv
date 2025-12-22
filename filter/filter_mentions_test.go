package filter

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
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
}

func TestMentionsFilter(t *testing.T) {
	ctx := context.Background()

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			MentionFilterMaxMentions:        internal.Pointer(3),
			MentionFilterMinPlaintextLength: internal.Pointer(5),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{MentionsFilterName},
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

	testCases := []mentionsTestCase{
		{
			EventType: "m.room.message",
			TestName:  "basic single mention is not spam",
			Body:      "Hi bob",
			Members: &members{
				UserIds:      []string{"@b:example.com"},
				DisplayNames: []string{"bob"},
			},
			WantNeutral: true,
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
			WantNeutral: true,
		},
		{
			EventType: "m.room.message",
			TestName:  "many mentions is spam",
			Body:      "Hi alice bob charlie",
			Members: &members{
				UserIds:      []string{"@a:example.com", "@b:example.com", "@c:example.com"},
				DisplayNames: []string{"alice", "bob", "charlie"},
			},
			WantNeutral: false,
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
			WantNeutral: false,
		},
		{
			EventType: "m.room.message",
			TestName:  "mentions for user ID is spam",
			Body:      "Hi @a:example.com @b:example.com @c:example.com ",
			Members: &members{
				UserIds:      []string{"@a:example.com", "@b:example.com", "@c:example.com"},
				DisplayNames: []string{"alice", "bob", "charlie"},
			},
			WantNeutral: false,
		},
		{
			EventType: "org.example.wrong_event_type_for_filter",
			TestName:  "no-ops on non-message events",
			Body:      "Hi bob",
			Members: &members{
				UserIds:      []string{"@b:example.com"},
				DisplayNames: []string{"bob"},
			},
			WantNeutral: true,
		},
		{
			EventType: "m.room.message",
			TestName:  "many low-character names are not spam",
			Body:      "a b c d e f g h i j k l m n o p q r s t u v w x y z",
			Members: &members{
				UserIds:      []string{"@notused:example.org"},
				DisplayNames: strings.Split("a b c d e f g h i j k l m n o p q r s t u v w x y z", " "),
			},
			WantNeutral: true,
		},
		{
			EventType: "m.room.message",
			TestName:  "detects spam for names for long enough names",
			Body:      "abcdef ghijkl mnopqr stuvwx yz",
			Members: &members{
				UserIds:      []string{"@notused:example.org"},
				DisplayNames: []string{"abcdef", "ghijkl", "mnopqr", "stuvwx", "yz"},
			},
			WantNeutral: false,
		},
	}

	roomId := "!foo:example.org"
	for _, tc := range testCases {
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
		assert.NoError(t, err, fmt.Sprintf("%s => wanted no error", tc.TestName))

		vecs, err := set.CheckEvent(ctx, event, nil)
		assert.NoError(t, err)
		if tc.WantNeutral {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam), fmt.Sprintf("%s => wanted spam vector of 0.5", tc.TestName))
			assert.Equal(t, 0.0, vecs.GetVector(classification.Mentions), fmt.Sprintf("%s => wanted mentions vector of 0.0", tc.TestName))
		} else {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam), fmt.Sprintf("%s => wanted spam vector of 1.0", tc.TestName))
			assert.Equal(t, 1.0, vecs.GetVector(classification.Mentions), fmt.Sprintf("%s => wanted mentions vector of 0.0", tc.TestName))
		}
	}
}
