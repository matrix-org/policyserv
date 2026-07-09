package filter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/media"
	"github.com/matrix-org/policyserv/notifiers"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func AssertCheckEvent(t *testing.T, set *Set, event gomatrixserverlib.PDU, expected *harms.ContentInfo) {
	info, err := set.CheckEvent(context.Background(), event, nil)
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, expected, info)
}

func AssertCheckText(t *testing.T, set *Set, eventWithBody gomatrixserverlib.PDU, expected *harms.ContentInfo) {
	body := gjson.Get(string(eventWithBody.Content()), "body").String()
	info, err := set.CheckText(context.Background(), body)
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, expected, info)
}

func AssertCheckTextAndEvent(t *testing.T, set *Set, eventWithBody gomatrixserverlib.PDU, expected *harms.ContentInfo) {
	AssertCheckText(t, set, eventWithBody, expected)
	AssertCheckEvent(t, set, eventWithBody, expected)
}

func TestNewSet(t *testing.T) {
	cnf := &SetConfig{
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{FixedFilterName},
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

	assert.Equal(t, len(cnf.Groups), len(set.groups))
	assert.Equal(t, len(cnf.Groups[0].EnabledNames), len(set.groups[0].filters))

	f, ok := set.groups[0].filters[0].(*FixedInstancedFilter)
	assert.True(t, ok)
	assert.NotNil(t, f)
	assert.Equal(t, f.Set, set)
}

func TestNewSetUnknownFilter(t *testing.T) {
	cnf := &SetConfig{
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{"this_is_not_a_real_filter_name"},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything is neutral by default in the test
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.ErrorContains(t, err, "error finding filter name")
	assert.Nil(t, set)
}

func TestNewSetErrorMaking(t *testing.T) {
	cnf := &SetConfig{
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{ErrorFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything is neutral by default in the test
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.ErrorContains(t, err, "error making filter")
	assert.Nil(t, set)
}

func TestSetCheckEvent(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		// We want to ensure we call *all* groups, so specify 2 to call
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // events start off neutral by default
		}, {
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassProhibited}, // we're going to test that harms are added
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	// Set up the fixed filters for testing
	f1 := set.groups[0].filters[0].(*FixedInstancedFilter)
	f1.T = t
	f2 := set.groups[1].filters[0].(*FixedInstancedFilter)
	f2.T = t

	// Set our expectations and return values
	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Content: make(map[string]any),
	})
	f1.Expect = &EventInput{Event: event, Medias: make([]*media.Item, 0)}
	f2.Expect = f1.Expect // same input
	f1.ReturnInfo = harms.ProhibitedContent(harms.SpamFlooding)
	f2.ReturnInfo = harms.ProhibitedContent(harms.SpamFraud) // this should be added on after the filters run

	AssertCheckEvent(t, set, event, harms.ProhibitedContent(harms.SpamFlooding, harms.SpamFraud))
}

func TestCheckEventWithErrorInGroup(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // events start off neutral by default
		}, {
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassProhibited}, // our fixed filter sets everything as prohibited
		}, {
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassProhibited}, // our fixed filter sets everything as prohibited
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Content: make(map[string]any),
	})
	inputs := []*EventInput{
		{
			Event:  event,
			Medias: make([]*media.Item, 0),
		}, {
			Event:  event,
			Medias: make([]*media.Item, 0),
		},
		nil, // should never be called
	}

	for i, group := range set.groups {
		for _, f := range group.filters {
			ff := f.(*FixedInstancedFilter)
			ff.T = t
			ff.ReturnInfo = harms.ProhibitedContent(harms.SpamFlooding)
			ff.Expect = inputs[i]
		}
	}
	errorFilter := set.groups[1].filters[0].(*FixedInstancedFilter)
	errorFilter.ReturnErr = errors.New("error within filter group")
	errorFilter.ReturnInfo = nil

	vecs, err := set.CheckEvent(context.Background(), event, nil)
	assert.ErrorContains(t, err, "error at group 1")
	assert.Nil(t, vecs)
}

func TestSetCheckText(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		// We want to ensure we call *all* groups, so specify 2 to call
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // content start off neutral by default
		}, {
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassProhibited}, // we're going to test that harms are added
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	// Set up the fixed filters for testing
	f1 := set.groups[0].filters[0].(*FixedInstancedFilter)
	f1.T = t
	f2 := set.groups[1].filters[0].(*FixedInstancedFilter)
	f2.T = t

	// Set our expectations and return values
	f1.ExpectText = "Hello world"
	f2.ExpectText = "Hello world"
	f1.ReturnInfo = harms.ProhibitedContent(harms.SpamFlooding)
	f2.ReturnInfo = harms.ProhibitedContent(harms.SpamFraud) // this should be added on after the filters run

	info, err := set.CheckText(context.Background(), "Hello world")
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.ProhibitedContent(harms.SpamFlooding, harms.SpamFraud), info)
}

func TestSetCheckTextWithErrorInGroup(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // events start off neutral by default
		}, {
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassProhibited}, // our fixed filter sets everything as prohibited
		}, {
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassProhibited}, // our fixed filter sets everything as prohibited
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()
	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	for i, group := range set.groups {
		for _, f := range group.filters {
			ff := f.(*FixedInstancedFilter)
			ff.T = t
			ff.ReturnInfo = harms.ProhibitedContent(harms.SpamFlooding)
			if i > 1 {
				ff.ReturnErr = errors.New("should never happen")
				ff.ExpectText = "if you see this in the test output, it broke"
			} else {
				ff.ExpectText = "Hello world"
			}
		}
	}
	errorFilter := set.groups[1].filters[0].(*FixedInstancedFilter)
	errorFilter.ReturnErr = errors.New("error within filter group")
	errorFilter.ReturnInfo = nil

	vecs, err := set.CheckText(context.Background(), "Hello world")
	assert.ErrorContains(t, err, "error at group 1")
	assert.Nil(t, vecs)
}

func TestCallsWebhook(t *testing.T) {
	t.Parallel()

	// Create a test server to receive webhooks
	calls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "/webhook", r.URL.Path)

		b, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		// We don't check the full output, just that some important bits are there
		assert.True(t, strings.Contains(string(b), "A user has had an event of theirs flagged as spam by policyserv"))
		assert.True(t, strings.Contains(string(b), "<b>Room ID:</b> <code>!foo:example.org</code> (<a href=\\\"https://matrix.to/#/!foo:example.org\\\">!foo:example.org</a>)"))
		assert.True(t, strings.Contains(string(b), "<b>Event ID:</b> <code>$test</code>"))
		assert.True(t, strings.Contains(string(b), "<b>User ID:</b> <code>@alice:example.org</code>"))

		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte("ok"))
		assert.NoError(t, err)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	parsedUrl, err := url.Parse(server.URL)
	assert.NoError(t, err)

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			WebhookUrl: internal.Pointer(server.URL + "/webhook"),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // starts as neutral, stays neutral until the next filter runs
		}, {
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral},
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	notifier, err := notifiers.NewWebhookMatrixNotifier(memStorage, 5, []string{parsedUrl.Host})
	assert.NoError(t, err)
	assert.NotNil(t, notifier)
	set, err := NewSet(cnf, memStorage, ps, notifier, nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	// Insert the community so the notifier works
	err = memStorage.UpsertCommunity(context.Background(), &storage.StoredCommunity{
		CommunityId: set.communityId,
		Config:      set.communityConfig,
	})
	assert.NoError(t, err)

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "hello @world",
		},
	})

	// The first group returns a neutral response and the second group returns spam. This is to ensure that the resulting
	// webhook shows the groups independently.

	for i := 0; i < 2; i++ {
		fixedFilter := set.groups[i].filters[0].(*FixedInstancedFilter)
		fixedFilter.T = t
		fixedFilter.Set = set
		fixedFilter.Expect = &EventInput{
			Event:  event,
			Medias: make([]*media.Item, 0),
		}
		fixedFilter.ReturnInfo = harms.NeutralContent()

		if i > 0 {
			fixedFilter.ReturnInfo = harms.ProhibitedContent(harms.SpamFlooding)
		}
	}

	info, err := set.CheckEvent(context.Background(), event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, info) // composition checked in other tests

	// Wait a bit so the goroutines can settle
	time.Sleep(1 * time.Second)

	// Check that the HTTP call was successful
	assert.Equal(t, 1, calls)
}

func TestCallsWebhookErrorNonFatal(t *testing.T) {
	t.Parallel()

	// Test that the filtering code doesn't return an error when the webhook does.

	// Create a test server to receive webhooks
	calls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError) // simulate an error
		_, err := w.Write([]byte("it broke"))
		assert.NoError(t, err)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	parsedUrl, err := url.Parse(server.URL)
	assert.NoError(t, err)

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			WebhookUrl: internal.Pointer(server.URL + "/webhook"),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything starts as neutral
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	notifier, err := notifiers.NewWebhookMatrixNotifier(memStorage, 5, []string{parsedUrl.Host})
	assert.NoError(t, err)
	assert.NotNil(t, notifier)
	set, err := NewSet(cnf, memStorage, ps, notifier, nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	// Insert the community so the notifier works
	err = memStorage.UpsertCommunity(context.Background(), &storage.StoredCommunity{
		CommunityId: set.communityId,
		Config:      set.communityConfig,
	})
	assert.NoError(t, err)

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"msgtype": "m.text",
			"body":    "hello @world",
		},
	})

	fixedFilter := set.groups[0].filters[0].(*FixedInstancedFilter)
	fixedFilter.T = t
	fixedFilter.Set = set
	fixedFilter.Expect = &EventInput{
		Event:  event,
		Medias: make([]*media.Item, 0),
	}
	fixedFilter.ReturnInfo = harms.ProhibitedContent(harms.SpamFlooding)

	vecs, err := set.CheckEvent(context.Background(), event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, vecs) // composition checked in other tests

	// Wait a bit so the goroutines can settle
	time.Sleep(1 * time.Second)

	// Check that the HTTP call was successful
	assert.Equal(t, 1, calls)
}

func TestExtractsMedia(t *testing.T) {
	t.Parallel()

	// Tests that media items are extracted from the event content when a downloader is supplied.

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		Groups: []*SetGroupConfig{{
			EnabledNames:          []string{FixedFilterName},
			CheckedContentClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything starts as neutral
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	downloader := test.MustMakeMediaDownloader(t)

	origin1 := "one.example.org"
	origin2 := "two.example.org"
	id1 := "abc"
	id2 := "def"

	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "org.example.event_type_doesnt_matter_in_this_test",
		Sender:  "@alice:example.org",
		Content: map[string]any{
			"url": fmt.Sprintf("mxc://%s/%s", origin1, id1),
			"info": map[string]any{
				"thumbnail_url": fmt.Sprintf("mxc://%s/%s", origin2, id2),
			},
		},
	})

	fixedFilter := set.groups[0].filters[0].(*FixedInstancedFilter)
	fixedFilter.T = t
	fixedFilter.Set = set
	fixedFilter.Expect = &EventInput{
		Event: event,
		Medias: []*media.Item{
			{
				Origin:  origin1,
				MediaId: id1,
			},
			{
				Origin:  origin2,
				MediaId: id2,
			},
		},
	}
	fixedFilter.ReturnInfo = harms.ProhibitedContent(harms.SpamFlooding)

	res, err := set.CheckEvent(context.Background(), event, downloader)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Now test that there are no media items extracted when no downloader is supplied.
	fixedFilter.Expect = &EventInput{
		Event:  event,
		Medias: make([]*media.Item, 0),
	}
	res, err = set.CheckEvent(context.Background(), event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, res)
}
