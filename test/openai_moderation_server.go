package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
)

// Dev note: Usually we'd write a dedicated test for utilities like this, however the entire functionality is covered by
// other tests using it, so it should be fine.

// KeywordSpammy - Used by tests to always flag a message as generic spam.
const KeywordSpammy = "MX_SPAMMY"

// KeywordSpammyCSAM - Used by tests to always flag a message as "containing CSAM".
const KeywordSpammyCSAM = "MX_SPAMMY_CSAM"

// KeywordNeutral - Used by tests to always flag a message as neutral ("not spammy").
const KeywordNeutral = "MX_NEUTRAL"

// KeywordIntentionalFail - Used by tests to always cause a 500 Internal Server Error response.
const KeywordIntentionalFail = "MX_INTENTIONAL_FAIL"

// MustMakeKeywordEvent - Creates a consistent event using the specified keyword
func MustMakeKeywordEvent(keyword string) gomatrixserverlib.PDU {
	return MustMakePDU(&BaseClientEvent{
		EventId: "$openai." + keyword,
		RoomId:  "!foo:example.org",
		Type:    "m.room.message",
		Sender:  "@user:example.org",
		Content: map[string]any{
			"msgtype":        "m.text",
			"body":           keyword + " | This is a message.",
			"format":         "org.matrix.custom.html",
			"formatted_body": "<b>" + keyword + "</b> | This is a message.",
		},
	})
}

// MakeOpenAIModerationServer - Creates a mock OpenAI Moderation API server for use in tests.
func MakeOpenAIModerationServer(t *testing.T, apiKey string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		if strings.Contains(req, KeywordSpammyCSAM) {
			modServerHandleKeywordSpammyCSAM(t, w, req)
		} else if strings.Contains(req, KeywordSpammy) {
			modServerHandleKeywordSpammy(t, w, req)
		} else if strings.Contains(req, KeywordNeutral) {
			modServerHandleKeywordNeutral(t, w, req)
		} else if strings.Contains(req, KeywordIntentionalFail) {
			modServerHandleKeywordIntentionalFail(t, w, req)
		} else {
			t.Fatalf("Unexpected request: %s", req)
		}
	}))
}

func assertInputMatchesKeyword(t *testing.T, keyword string, body string) {
	ev := MustMakeKeywordEvent(keyword)
	content := struct {
		Body          string `json:"body"`
		FormattedBody string `json:"formatted_body"`
	}{}
	err := json.Unmarshal(ev.Content(), &content)
	assert.NoError(t, err)

	inputs := make([]string, 2)
	for i, val := range []string{content.Body, content.FormattedBody} {
		inputs[i] = fmt.Sprintf(`{"input":"%s says: %s","model":"omni-moderation-latest"}`, ev.SenderID().ToUserID().String(), val)
	}

	assert.Contains(t, inputs, body)
}

func modServerHandleKeywordSpammyCSAM(t *testing.T, w http.ResponseWriter, body string) {
	assertInputMatchesKeyword(t, KeywordSpammyCSAM, body)

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
	b, err := json.Marshal(res)
	assert.NoError(t, err)
	_, _ = w.Write(b)
}

func modServerHandleKeywordSpammy(t *testing.T, w http.ResponseWriter, body string) {
	assertInputMatchesKeyword(t, KeywordSpammy, body)

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
	b, err := json.Marshal(res)
	assert.NoError(t, err)
	_, _ = w.Write(b)
}

func modServerHandleKeywordNeutral(t *testing.T, w http.ResponseWriter, body string) {
	assertInputMatchesKeyword(t, KeywordNeutral, body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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
	b, err := json.Marshal(res)
	assert.NoError(t, err)
	_, _ = w.Write(b)
}

func modServerHandleKeywordIntentionalFail(t *testing.T, w http.ResponseWriter, body string) {
	assertInputMatchesKeyword(t, KeywordIntentionalFail, body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError) // should prevent automatic retry from happening
	// This is a mock OpenAI API error
	_, _ = w.Write([]byte(`{"error":{"code": "X-ERROR","message":"Intentional fail","param":"x","type":"x"}}`))
}
