package filter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/media"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestNewSet(t *testing.T) {
	cnf := &SetConfig{
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0.3,
			MaximumSpamVectorValue: 0.8,
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	assert.Equal(t, set.GetStorage(), memStorage)

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
			EnabledNames:           []string{"this_is_not_a_real_filter_name"},
			MinimumSpamVectorValue: 0.3,
			MaximumSpamVectorValue: 0.8,
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
	assert.ErrorContains(t, err, "error finding filter name")
	assert.Nil(t, set)
}

func TestNewSetErrorMaking(t *testing.T) {
	cnf := &SetConfig{
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{ErrorFilterName},
			MinimumSpamVectorValue: 0.3,
			MaximumSpamVectorValue: 0.8,
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
	assert.ErrorContains(t, err, "error making filter")
	assert.Nil(t, set)
}

func TestSetCheckEvent(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		// We want to ensure we call *all* groups, so specify 2 to call
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
			MaximumSpamVectorValue: 1,
		}, {
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
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

	// Set up the fixed filters for testing
	f1 := set.groups[0].filters[0].(*FixedInstancedFilter)
	f1.T = t
	f2 := set.groups[1].filters[0].(*FixedInstancedFilter)
	f2.T = t

	// Set our expectations and return values
	// Note: internally, sets start with a 0.5 spam vector, so things get divided by 3 rather than 2
	event := test.MustMakePDU(&test.BaseClientEvent{
		RoomId:  "!foo:example.org",
		EventId: "$test",
		Type:    "m.room.message",
		Content: make(map[string]any),
	})
	f1.Expect = &EventInput{Event: event, Medias: make([]*media.Item, 0), IncrementalConfidenceVectors: confidence.Vectors{
		classification.Spam: 0.5,
	}}
	f1.ReturnClasses = []classification.Classification{
		classification.Spam,
		classification.Mentions,
	}
	f2.Expect = &EventInput{Event: event, Medias: make([]*media.Item, 0), IncrementalConfidenceVectors: confidence.Vectors{
		classification.Spam:     1.0,
		classification.Mentions: 1.0,
	}}
	f2.ReturnClasses = []classification.Classification{
		classification.Spam.Invert(),
		classification.Volumetric,
	}

	vecs, err := set.CheckEvent(context.Background(), event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, vecs)
	assert.Equal(t, confidence.Vectors{
		classification.Spam:       1.0,
		classification.Mentions:   1.0,
		classification.Volumetric: 1.0,
	}, vecs)
}

func TestCheckEventWithErrorInGroup(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
			MaximumSpamVectorValue: 1,
		}, {
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
			MaximumSpamVectorValue: 1.0,
		}, {
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
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
			IncrementalConfidenceVectors: confidence.Vectors{
				classification.Spam: 0.5,
			},
		}, {
			Event:  event,
			Medias: make([]*media.Item, 0),
			IncrementalConfidenceVectors: confidence.Vectors{
				classification.Spam: 1.0, // group 0 result
			},
		},
		nil, // should never be called
	}

	for i, group := range set.groups {
		for _, f := range group.filters {
			ff := f.(*FixedInstancedFilter)
			ff.T = t
			ff.ReturnClasses = []classification.Classification{classification.Spam}
			ff.Expect = inputs[i]
		}
	}
	errorFilter := set.groups[1].filters[0].(*FixedInstancedFilter)
	errorFilter.ReturnErr = errors.New("error within filter group")
	errorFilter.ReturnClasses = nil

	vecs, err := set.CheckEvent(context.Background(), event, nil)
	assert.ErrorContains(t, err, "error at group 1")
	assert.Nil(t, vecs)
}

func TestSetCheckText(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		// We want to ensure we call *all* groups, so specify 2 to call
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
			MaximumSpamVectorValue: 1,
		}, {
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
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

	// Set up the fixed filters for testing
	f1 := set.groups[0].filters[0].(*FixedInstancedFilter)
	f1.T = t
	f2 := set.groups[1].filters[0].(*FixedInstancedFilter)
	f2.T = t

	// Set our expectations and return values
	// Note: internally, sets start with a 0.5 spam vector, so things get divided by 3 rather than 2
	f1.ExpectText = "Hello world"
	f2.ExpectText = "Hello world"
	f1.ReturnClasses = []classification.Classification{
		classification.Spam,
		classification.Mentions,
	}
	f2.ReturnClasses = []classification.Classification{
		classification.Spam.Invert(),
		classification.Volumetric,
	}

	vecs, err := set.CheckText(context.Background(), "Hello world")
	assert.NoError(t, err)
	assert.NotNil(t, vecs)
	assert.Equal(t, confidence.Vectors{
		classification.Spam:       1.0,
		classification.Mentions:   1.0,
		classification.Volumetric: 1.0,
	}, vecs)
}

func TestSetCheckTextWithErrorInGroup(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
			MaximumSpamVectorValue: 1,
		}, {
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
			MaximumSpamVectorValue: 1.0,
		}, {
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0,
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

	for i, group := range set.groups {
		for _, f := range group.filters {
			ff := f.(*FixedInstancedFilter)
			ff.T = t
			ff.ReturnClasses = []classification.Classification{classification.Spam}
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
	errorFilter.ReturnClasses = nil

	vecs, err := set.CheckText(context.Background(), "Hello world")
	assert.ErrorContains(t, err, "error at group 1")
	assert.Nil(t, vecs)
}

func TestSetSpamThreshold(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			SpamThreshold: internal.Pointer(0.6),
		},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	assert.Equal(t, true, set.IsSpamResponse(context.Background(), confidence.Vectors{
		classification.Spam: 0.6, // exact match
	}))
	assert.Equal(t, true, set.IsSpamResponse(context.Background(), confidence.Vectors{
		classification.Spam: 0.7, // just above
	}))
	assert.Equal(t, false, set.IsSpamResponse(context.Background(), confidence.Vectors{
		classification.Spam: 0.5, // just below
	}))
	assert.Equal(t, false, set.IsSpamResponse(context.Background(), confidence.Vectors{
		classification.Spam: 0.0, // very much below
	}))
	assert.Equal(t, true, set.IsSpamResponse(context.Background(), confidence.Vectors{
		classification.Spam: 1.0, // very much above
	}))
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

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			SpamThreshold: internal.Pointer(0.6),
			WebhookUrl:    internal.Pointer(server.URL + "/webhook"),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}, {
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}},
		InstanceConfig: &config.InstanceConfig{
			AllowedWebhookDomains: []string{server.Listener.Addr().String()},
		},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

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
	// webhook shows the groups independently. This also makes writing the test easier because both groups will receive
	// the same initial (incremental) vectors.

	for i := 0; i < 2; i++ {
		fixedFilter := set.groups[i].filters[0].(*FixedInstancedFilter)
		fixedFilter.T = t
		fixedFilter.Set = set
		fixedFilter.Expect = &EventInput{
			Event:                        event,
			Medias:                       make([]*media.Item, 0),
			IncrementalConfidenceVectors: confidence.Vectors{classification.Spam: 0.5},
		}
		fixedFilter.ReturnClasses = []classification.Classification{}

		if i > 0 {
			fixedFilter.ReturnClasses = append(fixedFilter.ReturnClasses, classification.Spam, classification.Mentions)
		}
	}

	vecs, err := set.CheckEvent(context.Background(), event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, vecs) // composition checked in other tests

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

	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			SpamThreshold: internal.Pointer(0.6),
			WebhookUrl:    internal.Pointer(server.URL + "/webhook"),
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{FixedFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}},
		InstanceConfig: &config.InstanceConfig{
			AllowedWebhookDomains: []string{server.Listener.Addr().String()},
		},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

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
		Event:                        event,
		Medias:                       make([]*media.Item, 0),
		IncrementalConfidenceVectors: confidence.Vectors{classification.Spam: 0.5},
	}
	fixedFilter.ReturnClasses = []classification.Classification{classification.Spam}

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
			EnabledNames:           []string{FixedFilterName},
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
		Event:                        event,
		IncrementalConfidenceVectors: confidence.Vectors{classification.Spam: 0.5},
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
	fixedFilter.ReturnClasses = []classification.Classification{classification.Spam}

	res, err := set.CheckEvent(context.Background(), event, downloader)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Now test that there are no media items extracted when no downloader is supplied.
	fixedFilter.Expect = &EventInput{
		Event:                        event,
		IncrementalConfidenceVectors: confidence.Vectors{classification.Spam: 0.5},
		Medias:                       make([]*media.Item, 0),
	}
	res, err = set.CheckEvent(context.Background(), event, nil)
	assert.NoError(t, err)
	assert.NotNil(t, res)
}
