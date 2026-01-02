package pslib

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"text/template"
)

var punctuationRegex = regexp.MustCompile("[^\\w\\s]")
var whitespaceRegex = regexp.MustCompile("\\s")

var keywordTemplateFunctions = template.FuncMap{
	"ToLower":        strings.ToLower,
	"ToUpper":        strings.ToUpper,
	"StringContains": strings.Contains,
	"StrSliceContains": func(haystack []string, needle string) bool {
		return slices.Contains(haystack, needle)
	},
	"RemovePunctuation": func(s string) string {
		return punctuationRegex.ReplaceAllString(s, "")
	},
	"StrSlice": func(values ...string) []string {
		return values
	},
}

// KeywordTemplate - Used by the Keyword Template Filter to evaluate an input string for harms.
type KeywordTemplate struct {
	Name string
	tmpl *template.Template
}

// NewKeywordTemplate - Creates a new keyword template with the given name and text body.
func NewKeywordTemplate(name string, body string) (*KeywordTemplate, error) {
	tmpl, err := template.New(name).Funcs(keywordTemplateFunctions).Parse(body)
	if err != nil {
		return nil, err
	}
	return &KeywordTemplate{
		Name: name,
		tmpl: tmpl,
	}, nil
}

// IdentifyHarms - Takes an input string to evaluate it against the template. The harms are returned as MSC4387 identifiers.
// See https://github.com/matrix-org/matrix-spec-proposals/pull/4387 for details.
func (t *KeywordTemplate) IdentifyHarms(input string) ([]string, error) {
	templateInput := struct {
		BodyRaw   string
		BodyWords []string
	}{
		BodyRaw:   input,
		BodyWords: whitespaceRegex.Split(input, -1),
	}

	buf := bytes.NewBuffer(nil)
	err := t.tmpl.Execute(buf, templateInput)
	if err != nil {
		return nil, err
	}
	returnedHarms := whitespaceRegex.Split(buf.String(), -1)

	// We trim the harms because despite our best intentions, templates are bound to return *lots* of whitespace.
	trimmedHarms := make([]string, 0)
	for _, harm := range returnedHarms {
		harm = strings.TrimSpace(harm)
		if len(harm) == 0 {
			continue
		}
		trimmedHarms = append(trimmedHarms, harm)
	}

	return trimmedHarms, nil
}
