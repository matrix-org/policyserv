package filter

import (
	"context"
	"testing"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestMentionsFrequencyFilter(t *testing.T) {
	t.Parallel()

	cnf := &SetConfig{
		CommunityId: storage.NextId(), // use a real community ID to ensure we don't overflow in the pubsub layer
		CommunityConfig: &config.CommunityConfig{
			MentionFrequencyFilterMinPlaintextLength: internal.Pointer(5),
			MentionFrequencyFilterRateLimit:          internal.Pointer(5.0 / 60.0), // 5 mentions per minute
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{MentionsFrequencyFilterName},
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

	// Populate some users to mention
	targetUserIds := []string{"@one:example.org", "@two:example.org", "@three:example.org", "@four:example.org", "@five:example.org", "@six:example.org"}
	err = memStorage.SetUserIdsAndDisplayNamesByRoomId(context.Background(), "!foo:example.org", targetUserIds, make([]string, 0))
	assert.NoError(t, err)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
			"m.mentions": map[string]any{
				"user_ids": []string{targetUserIds[0], targetUserIds[1], targetUserIds[2]}, // 3 of 5 mentions
			},
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
			"m.mentions": map[string]any{
				"user_ids": []string{targetUserIds[3], targetUserIds[4], targetUserIds[5]}, // +3 more mentions
			},
		},
	})
	noopEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.wrong_event_type_for_filter",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
			"m.mentions": map[string]any{
				"user_ids": targetUserIds,
			},
		},
	})

	// No-op events shouldn't affect frequency. Because our rate limit is a single event, if this is handled improperly
	// then the next event will fail its test.
	AssertCheckEvent(t, set, noopEvent1, harms.NeutralContent())

	// Now we send an event that's in scope, but is technically going to be neutral. This is because we increment at a
	// different point, so we might "miss" the first spammy event.
	AssertCheckEvent(t, set, spammyEvent1, harms.NeutralContent())

	// Give a little bit of time for the notifier to settle
	time.Sleep(100 * time.Millisecond)

	// Now try to send another event that's in scope. This time it should exceed the rate limit as spam.
	AssertCheckEvent(t, set, spammyEvent2, harms.ProhibitedContent(harms.SpamFlooding))

	// Allow the goroutines to settle before concluding the test
	time.Sleep(100 * time.Millisecond)
}
