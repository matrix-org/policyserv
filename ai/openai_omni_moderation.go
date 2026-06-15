package ai

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/event"
	"github.com/matrix-org/policyserv/harms"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type OpenAIOmniModerationConfig struct {
	FailSecure bool
}

type OpenAIOmniModeration struct {
	// Implements Provider[*OpenAIOmniModerationConfig]

	client openai.Client
}

func NewOpenAIOmniModeration(cnf *config.InstanceConfig, additionalClientOptions ...option.RequestOption) (Provider[*OpenAIOmniModerationConfig], error) {
	apiKey := cnf.OpenAIApiKey
	if len(apiKey) == 0 {
		return nil, errors.New("api key not set")
	}
	options := append([]option.RequestOption{option.WithAPIKey(apiKey)}, additionalClientOptions...)
	client := openai.NewClient(options...)
	return &OpenAIOmniModeration{
		client: client,
	}, nil
}

func (m *OpenAIOmniModeration) CheckEvent(ctx context.Context, cnf *OpenAIOmniModerationConfig, input *Input) (*harms.ContentInfo, error) {
	messages, err := event.RenderToText(input.Event)
	if err != nil {
		return nil, err
	}
	for _, message := range messages {
		// Note: we don't want to log message contents in production
		log.Printf("[%s | %s] Message sent by %s", input.Event.EventID(), input.Event.RoomID(), input.Event.SenderID())
		res, err := m.client.Moderations.New(ctx, openai.ModerationNewParams{
			Model: openai.ModerationModelOmniModerationLatest,
			Input: openai.ModerationNewParamsInputUnion{
				OfString: openai.String(message),
			},
		})
		if err != nil {
			log.Printf("[%s | %s] Error checking message: %s", input.Event.EventID(), input.Event.RoomID(), err)
			if cnf.FailSecure {
				log.Printf("[%s | %s] Returning spam response to block events and discourage retries", input.Event.EventID(), input.Event.RoomID())
				return harms.ProhibitedContent(harms.OtherGeneral), nil
			} else {
				log.Printf("[%s | %s] Returning neutral response despite error, per config", input.Event.EventID(), input.Event.RoomID())
				return harms.NeutralContent(), nil
			}
		}
		for _, r := range res.Results {
			// Note: we compress JSON here because the OpenAI library tends to return *a lot* of redundant detail, including JSON with newlines in it.
			log.Printf("[%s | %s] Result for sender %s: Flagged=%t Flags=%s Scores=%s", input.Event.EventID(), input.Event.RoomID(), input.Event.SenderID(), r.Flagged, compressJsonResponse(r.Categories), compressJsonResponse(r.CategoryScores))
			if r.Flagged {
				harmIds := []harms.Harm{harms.SpamGeneral}
				if r.Categories.SexualMinors {
					harmIds = append(harmIds, harms.ChildSafetyCSAM)
				}
				return harms.ProhibitedContent(harmIds...), nil
			}
		}
	}
	return harms.NeutralContent(), nil
}

type compressible interface {
	RawJSON() string // same definition that's shared with the OpenAI response parts
}

func compressJsonResponse(target compressible) string {
	raw := target.RawJSON()

	val := make(map[string]any)
	err := json.Unmarshal([]byte(raw), &val)
	if err != nil {
		log.Printf("Non-fatal error compressing JSON. Using uncompressed instead. %s", err)
		return raw
	}
	b, err := json.Marshal(val)
	if err != nil {
		log.Printf("Non-fatal error compressing JSON. Using uncompressed instead. %s", err)
		return raw
	}

	return string(b)
}
