package filter

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type ScreenshotClassifier string

const ScreenshotClassifierScreenshot ScreenshotClassifier = "screenshot"
const ScreenshotClassifierPhoto ScreenshotClassifier = "photo"
const ScreenshotClassifierUnknown ScreenshotClassifier = "unknown"
const ScreenshotClassifierTooLarge ScreenshotClassifier = "too_large" // always causes a reject

const ScreenshotClassifierFilterName = "ScreenshotClassifierFilter"

func init() {
	mustRegister(ScreenshotClassifierFilterName, &ScreenshotClassifierFilter{})
}

type ScreenshotClassifierFilter struct {
}

func (s *ScreenshotClassifierFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedScreenshotClassifierFilter{
		set:        set,
		aiProvider: NewOpenAIScreenshotClassifierAIProvider(set.instanceConfig),
		inRoomIds:  set.instanceConfig.OpenAIAllowedRoomIds,
	}, nil
}

type InstancedScreenshotClassifierFilter struct {
	set        *Set
	aiProvider AIProvider // we use indirection because we can't (realistically) test that OpenAI is working
	inRoomIds  []string
}

func (f *InstancedScreenshotClassifierFilter) Name() string {
	return ScreenshotClassifierFilterName
}

func (f *InstancedScreenshotClassifierFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	//if !slices.Contains(f.inRoomIds, input.Event.RoomID().String()) {
	//	return nil, nil
	//}
	return f.aiProvider.CheckEvent(ctx, &aiFilterConfig{
		FailSecure: f.set.communityConfig.OpenAIFilterFailSecure,
	}, input)
}

type OpenAIScreenshotClassifierAIProvider struct {
	client openai.Client
}

func NewOpenAIScreenshotClassifierAIProvider(instanceConfig *config.InstanceConfig) *OpenAIScreenshotClassifierAIProvider {
	client := openai.NewClient(option.WithAPIKey(instanceConfig.OpenAIApiKey))
	return &OpenAIScreenshotClassifierAIProvider{
		client: client,
	}
}

func (p *OpenAIScreenshotClassifierAIProvider) CheckEvent(ctx context.Context, cnf *aiFilterConfig, input *Input) ([]classification.Classification, error) {
	if len(input.Medias) == 0 {
		return nil, nil // nothing to scan here
	}

	// Run the checks concurrently for all media items
	mediaClassifications := make([]ScreenshotClassifier, len(input.Medias))
	wg := sync.WaitGroup{}
	for i, media := range input.Medias {
		wg.Add(1)
		go func(i int, media *Media) {
			defer wg.Done()
			var err error
			mediaClassifications[i], err = p.classifyMedia(ctx, media)
			if err != nil {
				log.Printf("[%s | %s] Error classifying media %s: %s", input.Event.EventID(), input.Event.RoomID(), media.String(), err)
				if cnf.FailSecure {
					mediaClassifications[i] = ScreenshotClassifierTooLarge
				} else {
					mediaClassifications[i] = ScreenshotClassifierUnknown
				}
			}
		}(i, media)
	}

	// Wait for all the work to complete
	wg.Wait()

	// Process the responses
	log.Printf("[%s | %s] Screenshot classification results: %v", input.Event.EventID(), input.Event.RoomID(), mediaClassifications)
	allowedTypes := []ScreenshotClassifier{ScreenshotClassifierScreenshot} // TODO: Config
	isAllowed := false
	for _, response := range mediaClassifications {
		if response == ScreenshotClassifierTooLarge {
			isAllowed = false
			break // this is a security failure, so we don't want to continue
		} else if slices.Contains(allowedTypes, response) {
			isAllowed = true
		}
	}
	if isAllowed {
		return nil, nil
	}
	return []classification.Classification{classification.Spam}, nil
}

func (p *OpenAIScreenshotClassifierAIProvider) classifyMedia(ctx context.Context, media *Media) (ScreenshotClassifier, error) {
	b, err := media.Download()
	if err != nil {
		return ScreenshotClassifierUnknown, fmt.Errorf("error downloading media: %w", err)
	}
	if len(b) > 50*1024*1024 { // 50mb
		return ScreenshotClassifierTooLarge, nil
	}

	mimeType := http.DetectContentType(b)
	if !strings.HasPrefix(mimeType, "image/") {
		log.Printf("%s detected as %s (non-image), skipping", media.String(), mimeType)
		return ScreenshotClassifierUnknown, nil
	}

	resp, err := p.client.Responses.New(ctx, responses.ResponseNewParams{
		Model: "gpt-5-nano",
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: responses.ResponseInputParam{{
				OfInputMessage: &responses.ResponseInputItemMessageParam{
					Role: "user",
					Content: responses.ResponseInputMessageContentListParam{{
						OfInputText: &responses.ResponseInputTextParam{
							Text: "Only respond with 'PHOTO', 'SCREENSHOT', or 'UNKNOWN' respectively. What is this?",
						},
					}, {
						OfInputImage: &responses.ResponseInputImageParam{
							Type:     "input_image",
							ImageURL: openai.String(fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(b))),
						},
					}},
				},
			}},
		},
	})
	if err != nil {
		return ScreenshotClassifierUnknown, err
	}

	aiClass := strings.ToUpper(strings.TrimSpace(resp.OutputText()))
	log.Printf("Classified %s as '%s'", media.String(), aiClass)
	if strings.HasPrefix(aiClass, "PHOTO") {
		return ScreenshotClassifierPhoto, nil
	} else if strings.HasPrefix(aiClass, "SCREENSHOT") {
		return ScreenshotClassifierScreenshot, nil
	} else {
		return ScreenshotClassifierUnknown, nil
	}
}
