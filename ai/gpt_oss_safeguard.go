package ai

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/event"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type GptOssSafeguardConfig struct {
	FailSecure bool
}

type GptOssSafeguard struct {
	// Implements Provider[*GptOssSafeguardConfig]

	client          openai.Client
	reasoningEffort shared.ReasoningEffort
	modelName       string
}

func NewGptOssSafeguard(cnf *config.InstanceConfig, additionalClientOptions ...option.RequestOption) (Provider[*GptOssSafeguardConfig], error) {
	options := append([]option.RequestOption{option.WithBaseURL(cnf.GptOssSafeguardOpenAIApiUrl)}, additionalClientOptions...)
	client := openai.NewClient(options...)
	return &GptOssSafeguard{
		client:          client,
		reasoningEffort: shared.ReasoningEffort(cnf.GptOssSafeguardReasoningEffort),
		modelName:       cnf.GptOssSafeguardModelName,
	}, nil
}

func (m *GptOssSafeguard) CheckEvent(ctx context.Context, cnf *GptOssSafeguardConfig, input *Input) ([]classification.Classification, error) {
	messages, err := event.RenderToText(input.Event)
	if err != nil {
		return nil, err
	}
	for _, message := range messages {
		// Note: we don't want to log message contents in production
		log.Printf("[%s | %s] Message sent by %s", input.Event.EventID(), input.Event.RoomID(), input.Event.SenderID())
		res, err := m.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:           m.modelName,
			ReasoningEffort: m.reasoningEffort,
			Messages: []openai.ChatCompletionMessageParamUnion{
				{
					OfSystem: &openai.ChatCompletionSystemMessageParam{
						Role: "system",
						Content: openai.ChatCompletionSystemMessageParamContentUnion{
							OfString: openai.String(strings.TrimSpace(safeguardSystemPromptSpamPolicy)),
						},
					},
				},
				{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Role: "user",
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: openai.String(message),
						},
					},
				},
			},
		})
		if err != nil {
			log.Printf("[%s | %s] Error checking message: %s", input.Event.EventID(), input.Event.RoomID(), err)
			if cnf.FailSecure {
				log.Printf("[%s | %s] Returning spam response to block events and discourage retries", input.Event.EventID(), input.Event.RoomID())
				return []classification.Classification{classification.Spam, classification.Frequency}, nil
			} else {
				log.Printf("[%s | %s] Returning neutral response despite error, per config", input.Event.EventID(), input.Event.RoomID())
				return nil, nil
			}
		}
		for _, r := range res.Choices {
			reasoning := "<<not provided>>"
			field, ok := r.Message.JSON.ExtraFields["reasoning"]
			if ok { // Note: ideally we'd check `field.Valid()`, but seemingly it's always invalid for some reason
				reasoning = field.Raw()
			}

			violation := safeguardViolationResponse{}
			err = json.Unmarshal([]byte(strings.TrimSpace(r.Message.Content)), &violation)
			if err != nil {
				log.Printf("[%s | %s] Error parsing response from safeguard ('%s'): %s", input.Event.EventID(), input.Event.RoomID(), r.Message.Content, err)
				if cnf.FailSecure {
					return []classification.Classification{classification.Spam, classification.Frequency}, nil
				}
				continue
			}

			log.Printf("[%s | %s] Result for sender %s: %#v", input.Event.EventID(), input.Event.RoomID(), input.Event.SenderID(), violation)
			log.Printf("[%s | %s] Reasoning: %s", input.Event.EventID(), input.Event.RoomID(), reasoning)
			if violation.Class == safeguardClassSpammy {
				// TODO: Return further classifications depending on `violation.Rules`
				return []classification.Classification{classification.Spam}, nil
			}
		}
	}
	return nil, nil
}
