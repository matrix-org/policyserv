package filter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/matrix-org/policyserv/event"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/pslib"
)

const KeywordTemplateFilterName = "KeywordTemplateFilter"

func init() {
	mustRegister(KeywordTemplateFilterName, &KeywordTemplateFilter{})
}

type KeywordTemplateFilter struct {
}

func (k *KeywordTemplateFilter) MakeFor(set *Set) (Instanced, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	enabledTemplates := internal.Dereference(set.communityConfig.KeywordTemplateFilterTemplateNames)
	templates := make([]*pslib.KeywordTemplate, 0)

	for _, templateName := range enabledTemplates {
		raw, err := set.storage.GetKeywordTemplate(ctx, templateName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue // skip this template
			}
			return nil, err
		}

		tmpl, err := pslib.NewKeywordTemplate(templateName, raw.Body)
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
	templates []*pslib.KeywordTemplate
}

func (f *InstancedKeywordTemplateFilter) Name() string {
	return KeywordTemplateFilterName
}

func (f *InstancedKeywordTemplateFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	if input.Event.Type() != "m.room.message" {
		// no-op and return the same vectors
		return nil, nil
	}

	content := &event.MessageEventContent{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		// Probably not a string
		return nil, err
	}

	combinedBody := content.Body + " " + content.FormattedBody

	harms := make([]string, 0)
	for _, tmpl := range f.templates {
		log.Printf("[%s | %s] Checking template '%s'", input.Event.EventID(), input.Event.RoomID().String(), tmpl.Name)
		returnedHarms, err := tmpl.IdentifyHarms(combinedBody)
		if err != nil {
			return nil, err
		}
		if len(returnedHarms) > 0 {
			log.Printf("[%s | %s] Template '%s' matched: %v", input.Event.EventID(), input.Event.RoomID().String(), tmpl.Name, returnedHarms)
			harms = append(harms, returnedHarms...)
		} else {
			log.Printf("[%s | %s] Template '%s' matched nothing", input.Event.EventID(), input.Event.RoomID().String(), tmpl.Name)
		}
	}

	if len(harms) > 0 {
		// Our classification system doesn't (yet?) support MSC4387 harms, so just return "spam"
		return []classification.Classification{classification.Spam}, nil
	}
	return nil, nil
}
