package pslib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeywordTemplates(t *testing.T) {
	t.Parallel()

	body := `
	{{/* 
		This "template" returns various strings so we can assert that the return value is correct. This is because a
		"harm identifier" is nothing more than a delimited string returned by the template, so we can return as many
		identifiers as we like.
	*/}}

	{{/* Verify that the input is as expected */}}
	{{ .BodyRaw }}
	{{ range .BodyWords }}
		{{ . }}
	{{ end }}
	
	{{/* Confirm the ToUpper and ToLower functions work */}}
	{{ if eq "UPPER" (ToUpper "upper") }}
		TO_UPPER_WORKED
	{{ end }}
	{{ if eq "lower" (ToLower "LOWER") }}
		TO_LOWER_WORKED
	{{ end }}

	{{/* Confirm RemovePunctuation works (we don't test all possible punctuation, just a bunch of it) */}}
	{{ if eq "punctuation should be removed" (RemovePunctuation "!':;.,@?punctuation should be removed!':;.,@?") }}
		REMOVE_PUNCTUATION_WORKED
	{{ end }}

	{{/* Confirm StringContains works */}}
	{{ if StringContains "this is a test of the contains function" "test" }}
		STRING_CONTAINS_WORKED
	{{ end }}

	{{/* Confirm that string slices work */}}
	{{ range (StrSlice "one" "two" "three") }}
		{{/* we only need to test that one of the values works */}}
		{{ if eq . "two" }}
			STRSLICE_WORKED
		{{ end }}
	{{ end }}
	{{ $slice := StrSlice "one" "two" "three" }}
	{{ if StrSliceContains $slice "two" }}
		STRSLICE_CONTAINS_WORKED
	{{ end }}
	`

	name := "test_template"
	tmpl, err := NewKeywordTemplate(name, body)
	assert.NoError(t, err)
	assert.NotNil(t, tmpl)
	assert.Equal(t, name, tmpl.Name)

	harms, err := tmpl.IdentifyHarms("the_harms_input_should_be retained_as_two_words")
	assert.NoError(t, err)
	assert.Equal(t, []string{
		// "harms" are really just delimited/cleaned up values returned by the template, so look for that.

		// These two are from the input being tested as BodyWords and BodyRaw directly (testing that the input is correct)
		"the_harms_input_should_be",
		"retained_as_two_words",
		"the_harms_input_should_be",
		"retained_as_two_words",

		// These are function output tests.
		"TO_UPPER_WORKED",
		"TO_LOWER_WORKED",
		"REMOVE_PUNCTUATION_WORKED",
		"STRING_CONTAINS_WORKED",
		"STRSLICE_WORKED",
		"STRSLICE_CONTAINS_WORKED",
	}, harms)
}
