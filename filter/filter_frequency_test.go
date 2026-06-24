package filter

import (
	"testing"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestFrequencyFilter(t *testing.T) {
	t.Parallel()

	cnf := &SetConfig{
		CommunityId: storage.NextId(), // use a real community ID to ensure we don't overflow in the pubsub layer
		CommunityConfig: &config.CommunityConfig{
			FrequencyFilterEventTypes: &[]string{"m.room.message"},
			FrequencyFilterRateLimit:  internal.Pointer(1.0 / 60.0), // 1 message per minute
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{FrequencyFilterName},
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

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	noopEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.wrong_event_type_for_filter",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
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
