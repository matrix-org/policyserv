package filter

import (
	"context"
	"log"
	"slices"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/sashabaranov/go-openai"
)

const OpenAIFilterName = "OpenAIFilter"

func init() {
	mustRegister(OpenAIFilterName, &OpenAIFilter{})
}

type aiFilterConfig struct {
	FailSecure bool
}

type OpenAIFilter struct {
}

func (o *OpenAIFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedOpenAIFilter{
		set:        set,
		aiProvider: NewOpenAIModerationModelAIProvider(set.instanceConfig),
		inRoomIds:  set.instanceConfig.OpenAIAllowedRoomIds,
	}, nil
}

type InstancedOpenAIFilter struct {
	set        *Set
	aiProvider AIProvider // we use indirection because we can't (realistically) test that OpenAI is working
	inRoomIds  []string
}

func (f *InstancedOpenAIFilter) Name() string {
	return OpenAIFilterName
}

func (f *InstancedOpenAIFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	if !slices.Contains(f.inRoomIds, input.Event.RoomID().String()) {
		return nil, nil
	}
	return f.aiProvider.CheckEvent(ctx, &aiFilterConfig{
		FailSecure: f.set.communityConfig.OpenAIFilterFailSecure,
	}, input)
}

type AIProvider interface {
	CheckEvent(ctx context.Context, cnf *aiFilterConfig, input *Input) ([]classification.Classification, error)
}

type OpenAIModerationModelAIProvider struct {
	client *openai.Client
}

func NewOpenAIModerationModelAIProvider(instanceConfig *config.InstanceConfig) *OpenAIModerationModelAIProvider {
	client := openai.NewClient(instanceConfig.OpenAIApiKey)
	return &OpenAIModerationModelAIProvider{
		client: client,
	}
}

func (p *OpenAIModerationModelAIProvider) CheckEvent(ctx context.Context, cnf *aiFilterConfig, input *Input) ([]classification.Classification, error) {
	messages, err := renderEventToText(input.Event)
	if err != nil {
		return nil, err
	}
	for _, message := range messages {
		// Note: we don't want to log message contents in production
		log.Printf("[%s | %s] Message sent by %s", input.Event.EventID(), input.Event.RoomID(), input.Event.SenderID())
		res, err := p.client.Moderations(ctx, openai.ModerationRequest{
			Input: message,
			Model: openai.ModerationOmniLatest,
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
		for _, r := range res.Results {
			log.Printf("[%s | %s] Result for sender %s: %+v", input.Event.EventID(), input.Event.RoomID(), input.Event.SenderID(), r)
			if r.Flagged {
				flags := []classification.Classification{classification.Spam}
				if r.Categories.SexualMinors {
					flags = append(flags, classification.CSAM)
				}
				return flags, nil
			}
		}
	}
	return nil, nil
}
