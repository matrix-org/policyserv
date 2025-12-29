package filter

import (
	"context"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/test"
	"github.com/stretchr/testify/assert"
)

func TestKeywordTemplateFilter(t *testing.T) {
	cnf := &SetConfig{
		CommunityConfig: &config.CommunityConfig{
			// The filter should skip unknown template names, so we ask for one we won't be populating early on to
			// see if the code will explode at such a reference.
			KeywordTemplateFilterTemplateNames: &[]string{"example", "this_one_doesnt_exist_but_thats_okay"},
		},
		Groups: []*SetGroupConfig{{
			EnabledNames:           []string{KeywordTemplateFilterName},
			MinimumSpamVectorValue: 0.0,
			MaximumSpamVectorValue: 1.0,
		}},
	}
	memStorage := test.NewMemoryStorage(t)
	defer memStorage.Close()
	ps := test.NewMemoryPubsub(t)
	defer ps.Close()

	err := memStorage.UpsertKeywordTemplate(context.Background(), &storage.StoredKeywordTemplate{
		Name: "example", // the template name we do actually want to populate
		Body: `
			{{/* First, test that the functions exist. */}}
			{{ $unusedStr := .BodyRaw | ToUpper }}
			{{ $unusedStr := .BodyRaw | ToLower }}
			{{ $unusedStr := .BodyRaw | RemovePunctuation }}
			{{ $unusedBool := StringContains "foo" "bar" }}
			{{ $unusedBool := StrSliceContains .BodyWords "bar" }}
			{{ $unusedSlice := StrSlice "foo" "bar" }}

			{{/*
				Then, actually perform work. While doing this we try to verify all of the functions
				actually return something proper and useful. For example, we want to ensure that ToLower
				returns lower case strings.

				In a real filter, we wouldn't be doing simple "contains" checks, nor would we be necessarily
				be checking both BodyWords and BodyRaw with such simple logic.
			*/}}

			{{ $badWords := StrSlice "bad1" "bad2" "bad3" }}
			{{ range $bodyWord := .BodyWords }}
			  	{{ $bodyWord := $bodyWord | RemovePunctuation | ToLower }}
			  	{{ if StrSliceContains $badWords $bodyWord }}
					org.matrix.msc4387.spam
			  	{{ end }}
			{{ end }}
			{{ $rawBody := .BodyRaw | ToUpper }}
			{{ $badWords := StrSlice "BAD1" "BAD2" "BAD3" }}
			{{ range $badWord := $badWords }}
			  	{{ if StringContains $rawBody $badWord }}
					{{/* use "flooding" for variety, and so we can see which of these branches activated */}}
					org.matrix.msc4387.spam.flooding
				{{ end }}
			{{ end }}
        `,
	})
	assert.NoError(t, err)

	set, err := NewSet(cnf, memStorage, ps, test.MustMakeAuditQueue(5), nil)
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

	assertSpamVector := func(event gomatrixserverlib.PDU, isSpam bool) {
		vecs, err := set.CheckEvent(context.Background(), event, nil)
		assert.NoError(t, err)
		if isSpam {
			assert.Equal(t, 1.0, vecs.GetVector(classification.Spam))
		} else {
			// Because the filter doesn't flag things as "not spam", the seed value should survive
			assert.Equal(t, 0.5, vecs.GetVector(classification.Spam))
		}
	}
	assertSpamVector(spammyEvent1, true)
	assertSpamVector(spammyEvent2, true)
	assertSpamVector(spammyEvent3, true)
	assertSpamVector(neutralEvent, false)
}
