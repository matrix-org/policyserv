package filter

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
)

type keywordTemplateFilterInput struct {
	BodyRaw   string
	BodyWords []string
}

const KeywordTemplateFilterName = "KeywordTemplateFilter"

var punctuationRegex = regexp.MustCompile("[^\\w\\s]")

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

func init() {
	mustRegister(KeywordTemplateFilterName, &KeywordTemplateFilter{})
}

type KeywordTemplateFilter struct {
}

func (k *KeywordTemplateFilter) MakeFor(set *Set) (Instanced, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	enabledTemplates := internal.Dereference(set.communityConfig.KeywordTemplateFilterTemplateNames)
	templates := make([]*template.Template, 0)

	for _, templateName := range enabledTemplates {
		raw, err := set.storage.GetKeywordTemplate(ctx, templateName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue // skip this template
			}
			return nil, err
		}

		tmpl, err := template.New(templateName).Funcs(keywordTemplateFunctions).Parse(raw.Body)
		if err != nil {
			return nil, err
		}
		templates = append(templates, tmpl)
	}

	return &InstancedKeywordTemplateFilter{
		set:       set,
		templates: templates,
	}, nil
}

type InstancedKeywordTemplateFilter struct {
	set       *Set
	templates []*template.Template
}

func (f *InstancedKeywordTemplateFilter) Name() string {
	return KeywordTemplateFilterName
}

func (f *InstancedKeywordTemplateFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	if input.Event.Type() != "m.room.message" {
		// no-op and return the same vectors
		return nil, nil
	}

	content := &messageEventContent{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		// Probably not a string
		return nil, err
	}

	combinedBody := content.Body + " " + content.FormattedBody
	templateInput := keywordTemplateFilterInput{
		BodyRaw:   combinedBody,
		BodyWords: whitespaceRegex.Split(combinedBody, -1),
	}

	harms := make([]string, 0)
	for _, tmpl := range f.templates {
		log.Printf("[%s | %s] Checking template '%s'", input.Event.EventID(), input.Event.RoomID().String(), tmpl.Name())
		buf := bytes.NewBuffer(nil)
		err = tmpl.Execute(buf, templateInput)
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

		if len(trimmedHarms) > 0 {
			log.Printf("[%s | %s] Template '%s' matched: %v", input.Event.EventID(), input.Event.RoomID().String(), tmpl.Name(), trimmedHarms)
			harms = append(harms, trimmedHarms...)
		} else {
			log.Printf("[%s | %s] Template '%s' matched nothing", input.Event.EventID(), input.Event.RoomID().String(), tmpl.Name())
		}
	}

	if len(harms) > 0 {
		// Our classification system doesn't (yet?) support MSC4387 harms, so just return "spam"
		return []classification.Classification{classification.Spam}, nil
	}

	return nil, nil
}
