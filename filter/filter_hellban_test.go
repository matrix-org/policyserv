package filter

import (
	"context"
	"testing"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/media"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

const fastHellbanPrefilterName = "test_fast_hellban_prefilter"

func init() {
	mustRegister(fastHellbanPrefilterName, &TestFastHellbanPrefilter{})
}

type TestFastHellbanPrefilter struct{}

func (t *TestFastHellbanPrefilter) MakeFor(set *Set) (Instanced, error) {
	return newPrefilterHellban(set, 10*time.Second)
}

// Dev note: It's important that tests here (and across the filters package generally) use distinct
// community IDs to avoid interfering with each other.

func TestHellbanPrefilterDoesntEternallyExtend(t *testing.T) {
	// Here we test that the prefilter doesn't extend the hellban for a user indefinitely
	// when given multiple notifications to add such a spammy user to the cache.
	//
	// We use the special TestFastHellbanPrefilter to keep our test time down - it
	// is functionally the same as creating the prefilter normally, though without
	// needing to override the config (like in other tests)

	ctx := context.Background()
	cnf := &SetConfig{
		CommunityId:     "TestHellbanPrefilterDoesntEternallyExtend",
		CommunityConfig: &config.CommunityConfig{},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{fastHellbanPrefilterName},
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

	// Publish a bunch of times, a few seconds apart. Because the prefilter will hellban for 10 seconds,
	// we try to set about 6 seconds worth of notifications. so we can more easily detect if the first
	// one expires on time.
	spammerUserId := "@spam:example.org"
	for range 6 {
		assert.NoError(t, ps.Publish(ctx, pubsub.TopicHellban, mustEncodeHellban(cnf.CommunityId, spammerUserId)))
		time.Sleep(1 * time.Second)
	}

	// Now wait a few seconds to ensure the prefilter has had a chance to expire the first notification, but
	// not so long that the last one (~6 seconds after the first) would also expire.
	// A few extra seconds are added here to avoid test flakes.
	time.Sleep(6 * time.Second)

	// Now, the spammer should be able to send an event without being hellbanned.

	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.event_type_does_not_matter",
		Sender:  spammerUserId,
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})

	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
}

func TestHellbanPrefilter(t *testing.T) {
	ctx := context.Background()

	cnf := &SetConfig{
		CommunityId: "TestHellbanPrefilter",
		CommunityConfig: &config.CommunityConfig{
			HellbanPostfilterMinutes: internal.Pointer(10),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{HellbanPrefilterName},
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

	spammerUserId := "@spam:example.org"
	neutralUserId := "@neutral:example.org"
	err = ps.Publish(ctx, pubsub.TopicHellban, mustEncodeHellban(cnf.CommunityId, spammerUserId))
	assert.NoError(t, err)

	// While we're here, also test that the prefilter ignores publishes for other communities
	err = ps.Publish(ctx, pubsub.TopicHellban, mustEncodeHellban("unrelated_community", neutralUserId))
	assert.NoError(t, err)

	// This isn't great, but we need to ensure the prefilter has enough time to actually add
	// the user ID to its internal cache. This happens within milliseconds, but slower machines
	// may be slower.
	//
	// We can't "just" grab a second subscription to the pubsub layer because all subscribers
	// are called concurrently. This would cause effectively the same problem, with more code.
	time.Sleep(1 * time.Second)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.event_type_does_not_matter",
		Sender:  spammerUserId,
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.event_type_does_not_matter",
		Sender:  neutralUserId,
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})

	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamFlooding))
	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
}

func TestHellbanPostfilter(t *testing.T) {
	ctx := context.Background()

	cnf := &SetConfig{
		CommunityId:     "TestHellbanPostfilter",
		CommunityConfig: &config.CommunityConfig{},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything starts as neutral by default in the test
		}, {
			EnabledNames:          []string{HellbanPostfilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassProhibited}, // but we only want to detect spam for the postfilter test
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	fixedFilter := set.groups[0].filters[0].(*FixedInstancedFilter)
	fixedFilter.T = t
	fixedFilter.Set = set

	spammerUserId := "@spam:example.org"
	subCh, err := ps.Subscribe(ctx, pubsub.TopicHellban)
	assert.NoError(t, err)
	assert.NotNil(t, subCh)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.event_type_does_not_matter",
		Sender:  spammerUserId,
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})
	neutralEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.event_type_does_not_matter",
		Sender:  "@neutral:example.org",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})

	// Check the spammy event first (should cause a hellban)
	fixedFilter.Expect = &EventInput{
		Event:  spammyEvent1,
		Medias: make([]*media.Item, 0),
	}
	fixedFilter.ReturnInfo = harms.ProhibitedContent(harms.SpamGeneral)
	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamGeneral, harms.SpamFlooding))
	select {
	case recv := <-subCh:
		assert.Equal(t, mustEncodeHellban(cnf.CommunityId, spammerUserId), recv)
	case <-time.After(5 * time.Second): // if after a little bit we haven't heard anything, fail
		assert.Fail(t, "didn't receive a subscription event")
	}

	// Neutral events shouldn't cause a hellban
	fixedFilter.Expect.Event = neutralEvent1
	fixedFilter.ReturnInfo = harms.NeutralContent()
	AssertCheckEvent(t, set, neutralEvent1, harms.NeutralContent())
	select {
	case <-subCh:
		assert.Fail(t, "should not have received a subscription event")
	case <-time.After(1 * time.Second): // we use 1 second for the same reason as the prefilter above
		// passing case - we want this to happen
	}
}

func TestHellbanFiltersCombined(t *testing.T) {
	ctx := context.Background()

	cnf := &SetConfig{
		CommunityId: "TestHellbanFiltersCombined",
		CommunityConfig: &config.CommunityConfig{
			HellbanPostfilterMinutes: internal.Pointer(10),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{HellbanPrefilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything starts as neutral by default in the test
		}, {
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // for later: don't run on spammy events
		}, {
			EnabledNames:          []string{HellbanPostfilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassProhibited}, // *only* run on spammy events
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	fixedFilter := set.groups[1].filters[0].(*FixedInstancedFilter)
	fixedFilter.T = t
	fixedFilter.Set = set

	spammerUserId := "@spam:example.org"
	subCh, err := ps.Subscribe(ctx, pubsub.TopicHellban)
	assert.NoError(t, err)
	assert.NotNil(t, subCh)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "org.example.event_type_does_not_matter",
		Sender:  spammerUserId,
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})

	// The rough sequence of events should be:
	// 1. Event passes through prefilter untouched (no-op)
	// 2. Event hits fixed filter, which flags it as spam
	// 3. Event gets processed by postfilter as spammy
	// 4. Prefilter picks up the postfilter's hellban designation
	// 5. Sending the event again, event hits prefilter as spammy
	// 6. Fixed filter declares it as spam still
	// 7. The postfilter will still be triggered due to set config, but the prefilter should ignore the double hellban.
	//    We don't test this here, but it'll be tested in the prefilter test.
	//
	// We don't test a no-op event through this because those are tested individually
	// at each filter's specific test. The round-tripping is to ensure the cache is
	// updated, not that the filters work.

	// Step 2 prep
	fixedFilter.Expect = &EventInput{
		Event:  spammyEvent1,
		Medias: make([]*media.Item, 0),
	}
	// We set a media harm so we can detect that the filter ran
	fixedFilter.ReturnInfo = harms.ProhibitedContent(harms.SpamGeneral, harms.PolicyservMedia)

	// Invoke steps 1 through 3
	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamGeneral, harms.SpamFlooding, harms.PolicyservMedia))
	select {
	case recv := <-subCh:
		assert.Equal(t, mustEncodeHellban(cnf.CommunityId, spammerUserId), recv)
	case <-time.After(5 * time.Second): // if after a little bit we haven't heard anything, fail
		assert.Fail(t, "didn't receive a subscription event")
	}

	// We need to wait a bit to ensure the cache populates, per prefilter test
	time.Sleep(1 * time.Second)

	// Step 6 prep
	fixedFilter.Expect = &EventInput{
		Event:  spammyEvent1,
		Medias: make([]*media.Item, 0),
	}

	// Invoke steps 5 and 6 (step 4 is implied by the high spam figure)
	// Note: "media" shouldn't appear here because of the RunOnClasses config. We expect Flooding from the prefilter.
	AssertCheckEvent(t, set, spammyEvent1, harms.ProhibitedContent(harms.SpamFlooding))

	// Step 7 states that we should (probably) get a subscription event
	select {
	case recv := <-subCh:
		assert.Equal(t, mustEncodeHellban(cnf.CommunityId, spammerUserId), recv)
	case <-time.After(5 * time.Second): // if after a little bit we haven't heard anything, fail
		assert.Fail(t, "didn't receive a subscription event")
	}
}

func TestHellbanEncodingReversible(t *testing.T) {
	t.Parallel()

	communityId := "TestHellbanEncodingReversible"
	userId := "@spammer:example.org"

	encoded := mustEncodeHellban(communityId, userId)
	assert.NotEmpty(t, encoded)

	decodedCommunityId, decodedUserId := mustDecodeHellban(encoded)
	assert.Equal(t, communityId, decodedCommunityId)
	assert.Equal(t, userId, decodedUserId)
}
