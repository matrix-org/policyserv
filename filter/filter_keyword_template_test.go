package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestKeywordTemplateFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			// The filter should skip unknown template names, so we ask for one we won't be populating early on to
			// see if the code will explode at such a reference.
			// Note: "example" is here to ensure the default for KeywordTemplateFilterUseFullEvent is false. If it was true,
			// the filter would pick up on the "example.org" in the various IDs.
			KeywordTemplateFilterTemplateNames: &[]string{"example", "this_one_doesnt_exist_but_thats_okay"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{KeywordTemplateFilterName},
			RunOnClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything is neutral by default in the test
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	err := memStorage.UpsertKeywordTemplate(context.Background(), &storage.StoredKeywordTemplate{
		Name: "example", // the template name we do actually want to populate
		Body: `
			{{/*
				In a real filter, we wouldn't be doing simple "contains" checks, nor would we be necessarily
				be checking both BodyWords and BodyRaw with such simple logic.
			*/}}

			{{ $badWords := StrSlice "bad1" "bad2" "bad3" }}
			{{ range $bodyWord := .BodyWords }}
			  	{{ $bodyWord := $bodyWord | RemovePunctuation | ToLower }}
			  	{{ if StrSliceContains $badWords $bodyWord }}
					org.example.keyword.spam
			  	{{ end }}
			{{ end }}
			{{ $rawBody := .BodyRaw | ToUpper }}
			{{ $badWords := StrSlice "BAD1" "BAD2" "BAD3" }}
			{{ range $badWord := $badWords }}
			  	{{ if StringContains $rawBody $badWord }}
					{{/* use "flooding" for variety, and so we can see which of these branches activated */}}
					org.example.keyword.spam.flooding
				{{ end }}
			{{ end }}
        `,
	})
	assert.NoError(t, err)

	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			// This also tests punctuation getting in the way.
			"body": "this has a bad word, bad2, in it",
		},
	})
	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "LET'S TEST CASE SENSITIVITY TOO: BAD3",
		},
	})
	spammyEvent3 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body":           "the body is fine, but...",
			"formatted_body": "... the HTML is not <b>and contains bad1 in it</b>",
		},
	})
	neutralEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Content: map[string]any{
			"body": "this is not spam", // doesn't use `badWords`
		},
	})
	noopEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$noop",
		RoomId:  "!foo:example.org",
		Type:    "org.example.wrong_event_type_for_filter",
		Content: map[string]any{
			"body": "doesn't matter",
		},
	})

	AssertCheckTextAndEvent(t, set, spammyEvent1, harms.ProhibitedContent("org.example.keyword.spam", "org.example.keyword.spam.flooding"))
	AssertCheckTextAndEvent(t, set, spammyEvent2, harms.ProhibitedContent("org.example.keyword.spam", "org.example.keyword.spam.flooding"))
	AssertCheckEvent(t, set, spammyEvent3, harms.ProhibitedContent("org.example.keyword.spam", "org.example.keyword.spam.flooding")) // skip text check because it's HTML and we're not extracting that
	AssertCheckTextAndEvent(t, set, neutralEvent, harms.NeutralContent())
	AssertCheckEvent(t, set, noopEvent, harms.NeutralContent()) // skip text because text isn't aware of event types
}

func TestKeywordTemplateFilterWithFullEvent(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			KeywordTemplateFilterTemplateNames: &[]string{"example"},
			KeywordTemplateFilterUseFullEvent:  internal.Pointer(true), // this is what we're testing
		},
		Groups: []*SetGroupConfig{{
			EnabledNames: []string{KeywordTemplateFilterName},
			RunOnClasses: []harms.ContentClass{harms.ContentClassNeutral}, // everything is neutral by default in the test
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	err := memStorage.UpsertKeywordTemplate(context.Background(), &storage.StoredKeywordTemplate{
		Name: "example", // the template name from the config
		Body: `
			{{/* In a real filter, we wouldn't be doing simple "contains" checks. */}}
			{{ if StringContains .BodyRaw "user_id_has_the_keyword_instead" }}
				org.example.keyword.spam
			{{ end }}
        `,
	})
	assert.NoError(t, err)

	set, err := NewSet(cnf, memStorage, ps, test.NewMatrixNotifier(t), nil)
	assert.NoError(t, err)
	assert.NotNil(t, set)

	spammyEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@the_user_id_has_the_keyword_instead_of_the_event_content:example.org",
		Content: map[string]any{
			"body": "not applicable",
		},
	})

	AssertCheckEvent(t, set, spammyEvent, harms.ProhibitedContent("org.example.keyword.spam"))
}
