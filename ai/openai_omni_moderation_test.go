package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/test"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/assert"
)

func TestOpenAIOmniModeration(t *testing.T) {
	t.Parallel()

	// Caution: this is a long test. It mocks the OpenAI API to drive our Omni provider through 4 test cases:
	//  1. Regular spammy message (according to omni)
	//  2. Spammy message flagged for CSAM by omni
	//  3. Neutral message (according to omni)
	//  4. Simulates a server error to test `FailSecure` in both operation modes

	// We create our own HTTP client to intercept and act as the OpenAI API
	apiKey := "not_a_real_key"
	mockApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Header.Get("Authorization"), "Bearer "+apiKey)

		// Dev note: this HTTP handler is sensitive to changes in the OpenAI library. If it makes additional
		// calls ahead of the moderation test or changes what it supplies as a request body, then this test
		// will suddenly start failing. It's recommended to make changes to the Omni provider code separate
		// from OpenAI library upgrades to detect these failures more easily.

		assert.Equal(t, r.URL.Path, "/moderations") // we only handle Moderations API stuff here

		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err) // "should never happen"
		}
		req := string(b)

		// We look for keywords that change our behaviour as the mock API. These keywords should only be present
		// in the message text from an event.
		if strings.Contains(req, "MX_SPAMMY_CSAM") { // specifically flagged as CSAM response
			assert.Contains(t, []string{
				"{\"input\":\"@spammer:example.org says: this is a spammy event which should be classified as CSAM|MX_SPAMMY_CSAM\",\"model\":\"omni-moderation-latest\"}",
				"{\"input\":\"@spammer:example.org says: <b>this is a spammy event which should be classified as CSAM</b>|MX_SPAMMY_CSAM\",\"model\":\"omni-moderation-latest\"}",
			}, req)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			res := openai.ModerationNewResponse{
				ID:    "1",
				Model: openai.ModerationModelOmniModerationLatest,
				Results: []openai.Moderation{{
					Flagged: true, // flagged, and...
					Categories: openai.ModerationCategories{
						SexualMinors: true, // ... is detected as CSAM
					},
					CategoryScores: openai.ModerationCategoryScores{
						SexualMinors: 1.0,
					},
					CategoryAppliedInputTypes: openai.ModerationCategoryAppliedInputTypes{
						SexualMinors: []string{"text"},
					},
				}},
			}
			b, err = json.Marshal(res)
			if err != nil {
				t.Fatal(err) // "should never happen"
			}
			_, _ = w.Write(b)
		} else if strings.Contains(req, "MX_SPAMMY") { // generic spam/flagged response
			assert.Contains(t, []string{
				"{\"input\":\"@spammer:example.org says: this is a spammy event|MX_SPAMMY\",\"model\":\"omni-moderation-latest\"}",
				"{\"input\":\"@spammer:example.org says: <b>this is a spammy event</b>|MX_SPAMMY\",\"model\":\"omni-moderation-latest\"}",
			}, req)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			res := openai.ModerationNewResponse{
				ID:    "1",
				Model: openai.ModerationModelOmniModerationLatest,
				Results: []openai.Moderation{{
					Flagged: true, // flagged, but...
					Categories: openai.ModerationCategories{
						SexualMinors: false, // ... not CSAM to avoid accidentally causing that code to activate
					},
					CategoryScores: openai.ModerationCategoryScores{
						SexualMinors: 0.0,
					},
					CategoryAppliedInputTypes: openai.ModerationCategoryAppliedInputTypes{
						SexualMinors: []string{"text"},
					},
				}},
			}
			b, err = json.Marshal(res)
			if err != nil {
				t.Fatal(err) // "should never happen"
			}
			_, _ = w.Write(b)
		} else if strings.Contains(req, "MX_NEUTRAL") { // generic not-flagged response
			assert.Contains(t, []string{
				"{\"input\":\"@neutral:example.org says: this is a neutral event|MX_NEUTRAL\",\"model\":\"omni-moderation-latest\"}",
				"{\"input\":\"@neutral:example.org says: <b>this is a neutral event</b>|MX_NEUTRAL\",\"model\":\"omni-moderation-latest\"}",
			}, req)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Pretty much the same as above, but this time with CSAM flags off to avoid that causing an activation
			res := openai.ModerationNewResponse{
				ID:    "1",
				Model: openai.ModerationModelOmniModerationLatest,
				Results: []openai.Moderation{{
					Flagged:                   false, // "not flagged"
					Categories:                openai.ModerationCategories{},
					CategoryScores:            openai.ModerationCategoryScores{},
					CategoryAppliedInputTypes: openai.ModerationCategoryAppliedInputTypes{},
				}},
			}
			b, err = json.Marshal(res)
			if err != nil {
				t.Fatal(err) // "should never happen"
			}
			_, _ = w.Write(b)
		} else if strings.Contains(req, "MX_INTENTIONAL_FAIL") { // intentionally explode to test FailSecure
			assert.Contains(t, []string{
				"{\"input\":\"@fail:example.org says: this event will test the FailSecure flag|MX_INTENTIONAL_FAIL\",\"model\":\"omni-moderation-latest\"}",
				"{\"input\":\"@fail:example.org says: <b>this event will test the FailSecure flag</b>|MX_INTENTIONAL_FAIL\",\"model\":\"omni-moderation-latest\"}",
			}, req)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError) // should prevent automatic retry from happening
			// This is a mock OpenAI API error
			_, _ = w.Write([]byte(`{"error":{"code": "X-ERROR","message":"Intentional fail","param":"x","type":"x"}}`))
		}
	}))
	defer mockApi.Close()
	client := mockApi.Client() // get a client instance that trusts the mockApi certificate

	// Create the provider
	provider, err := NewOpenAIOmniModeration(
		&config.InstanceConfig{OpenAIApiKey: apiKey},
		option.WithHTTPClient(client),
		option.WithBaseURL(mockApi.URL),
	)
	assert.NoError(t, err)
	assert.NotNil(t, provider)

	spammyEvent1 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam1",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@spammer:example.org",
		Content: map[string]any{
			"body":           "this is a spammy event|MX_SPAMMY",
			"msgtype":        "m.text",
			"format":         "org.matrix.custom.html",
			"formatted_body": "<b>this is a spammy event</b>|MX_SPAMMY",
		},
	})
	ret, err := provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: true}, &Input{Event: spammyEvent1})
	assert.NoError(t, err)
	assert.Equal(t, []classification.Classification{
		classification.Spam,
	}, ret)

	spammyEvent2 := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$spam2",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@spammer:example.org",
		Content: map[string]any{
			"body":           "this is a spammy event which should be classified as CSAM|MX_SPAMMY_CSAM",
			"msgtype":        "m.text",
			"format":         "org.matrix.custom.html",
			"formatted_body": "<b>this is a spammy event which should be classified as CSAM</b>|MX_SPAMMY_CSAM",
		},
	})
	ret, err = provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: true}, &Input{Event: spammyEvent2})
	assert.NoError(t, err)
	assert.Equal(t, []classification.Classification{
		classification.Spam,
		classification.CSAM, // should have been detected
	}, ret)

	neutralEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$neutral",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@neutral:example.org",
		Content: map[string]any{
			"body":           "this is a neutral event|MX_NEUTRAL",
			"msgtype":        "m.text",
			"format":         "org.matrix.custom.html",
			"formatted_body": "<b>this is a neutral event</b>|MX_NEUTRAL",
		},
	})
	ret, err = provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: true}, &Input{Event: neutralEvent})
	assert.NoError(t, err)
	assert.Nil(t, ret) // no classifications

	failEvent := test.MustMakePDU(&test.BaseClientEvent{
		EventId: "$fail",
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@fail:example.org",
		Content: map[string]any{
			"body":           "this event will test the FailSecure flag|MX_INTENTIONAL_FAIL",
			"msgtype":        "m.text",
			"format":         "org.matrix.custom.html",
			"formatted_body": "<b>this event will test the FailSecure flag</b>|MX_INTENTIONAL_FAIL",
		},
	})

	// First test that when FailSecure: true we return a spam classification
	ret, err = provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: true}, &Input{Event: failEvent})
	assert.NoError(t, err)
	assert.Equal(t, []classification.Classification{
		classification.Spam,
		classification.Frequency, // also added by the provider
	}, ret)

	// Now when FailSecure: false, we should get no classifications (but also no errors)
	ret, err = provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: false}, &Input{Event: failEvent})
	assert.NoError(t, err)
	assert.Nil(t, ret)
}
