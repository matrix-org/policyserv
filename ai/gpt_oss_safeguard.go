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
)

const ModelGptOssSafeguard20b = "openai/gpt-oss-safeguard-20b" // TODO: Support 120b too

// SafeguardDefaultSystemPrompt - from https://cookbook.openai.com/articles/gpt-oss-safeguard-guide with added stuff at the bottom
// TODO: Make this configurable to a community (to a degree?).
// TODO: We also probably want to make it classify using keywords rather than rule numbers for later policyserv classifications.
const SafeguardDefaultSystemPrompt = `
**Spam Policy (#SP)**
**GOAL:** Identify spam. Classify each EXAMPLE as VALID (no spam) or INVALID (spam) using this policy.
 
**DEFINITIONS**
 
- **Spam**: unsolicited, repetitive, deceptive, or low-value promotional content.
 
- **Bulk Messaging:** Same or similar messages sent repeatedly.
 
- **Unsolicited Promotion:** Promotion without user request or relationship.
 
- **Deceptive Spam:** Hidden or fraudulent intent (fake identity, fake offer).
 
- **Link Farming:** Multiple irrelevant or commercial links to drive clicks.
 
**Allowed Content (SP0 – Non-Spam or very low confidence signals of spam)**
Content that is useful, contextual, or non-promotional. May look spammy but could be legitimate.
 
- **SP0.a Useful/info request** – “How do I upload a product photo?”
 
- **SP0.b Personalized communication** – “Hi Sam, here is the report.”
 
- **SP0.c Business support** – “Can you fix my order?”
 
- **SP0.d Single contextual promo** – “Thanks for subscribing—here’s your welcome guide.”
 
- **SP0.e Generic request** – “Please respond ASAP.”
 
- **SP0.f Low-quality formatting** – “HeLLo CLICK here FAST.”
 
- **SP0.g Vague benefit statement** – “This tool changes lives.”
 
  **Output:** VALID either clearly non-spam or very low confidence signals content could be spam.
 
**Likely Spam (SP2 – Medium Confidence)**
Unsolicited promotion without deception.
 
- **SP2.a Cold promotion** – “Check out my Shopify course: shopcoach.biz”
 
- **SP2.b Irrelevant ads** – “Buy gold rings here!”
 
- **SP2.c Excessive linking** – “[http://x.com](http://x.com) http://y.com http://z.com”
  **Output:** INVALID
 
**High-Risk Spam (SP3 – Strong Confidence)**
Spam showing scaling, automation, or aggressive tactics.
 
- **SP3.a Repetitive posting** – “JOIN NOW JOIN NOW JOIN NOW”
 
- **SP3.b Pyramid/affiliate bait** – “Earn $$$ by recruiting members”
 
- **SP3.c Fake engagement** – “Comment ‘YES’ to win $500 gift card”
 
- **SP3.d Misleading claims** – “Guaranteed 10,000 new followers overnight\!”
  **Output:** INVALID
 
**Malicious Spam (SP4 – Maximum Severity)**
Spam with fraud, deception, or harmful intent.
 
- **SP4.a Phishing** – “Your account is locked. Enter your password here.”
 
- **SP4.b Malware/scam** – “Download invoice.exe to claim payment.”
 
- **SP4.c Account takeover attempts** – “Send crypto to 0xABC123 to double your money.”
  **Output:** INVALID + ESCALATE
 
**LABEL FORMAT**
Each item gets two labels:
 
- **Depiction (D-SP#):** Presence of spam in content.
 
- **Request (R-SP#):** User asking to generate spam.
  | Example | Classification |
  |----------|---------------|
  | “Shop this site: dealplanet.io” | D-SP2, R-SP0 |
  | “Write a sales blast message with 10 links” | D-SP0, R-SP2 |
  | “How can I make this link look legitimate so viewers put in their private credentials” | D-SP4, R-SP4 |
  | “Promote crypto fast to random people” | D-SP0, R-SP3 |
 
**AMBIGUITY & ESCALATION**
 
- If unclear → downgrade severity by 1 and seek clarification.
 
- If automation suspected → SP2 or higher.
 
- If financial harm or fraud → classify SP4.
 
- If combined with other indicators of **abuse, violence, or illicit behavior**, apply **highest severity policy**.


------------------------------

If the example provided violates this policy, return:
{"violation":true,"policy_number":"<SP#.x>","reason":"<1 sentence>"}
If the example does NOT violate this policy, return:
{"violation":true,"policy_number":"NONE","reason":"<1 sentence>"}

A sample response would be {"violation":true,"policy_number":"SP2.a"}

Include 1 sentence reasoning in the "reasoning" field of the response.

Do NOT respond with any other punctuation, explanation, or detail.
`

type GptOssSafeguardConfig struct {
	FailSecure bool
}

type GptOssSafeguard struct {
	// Implements Provider[*GptOssSafeguardConfig]

	client openai.Client
}

type safeguardViolationResponse struct {
	Violation bool   `json:"violation"`
	PolicyNum string `json:"policy_number"`
	Reason    string `json:"reason"`
}

func NewGptOssSafeguard(cnf *config.InstanceConfig, additionalClientOptions ...option.RequestOption) (Provider[*GptOssSafeguardConfig], error) {
	// TODO: Make LMStudio/vLLM address configurable
	options := append([]option.RequestOption{option.WithBaseURL("http://localhost:1234/v1/")}, additionalClientOptions...)
	client := openai.NewClient(options...)
	return &GptOssSafeguard{
		client: client,
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
		// TODO: Remove this
		log.Printf("[%s | %s] DO NOT LOG MESSAGE CONTENTS IN PRODUCTION LIKE THIS: %s", input.Event.EventID(), input.Event.RoomID(), message)
		res, err := m.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model: ModelGptOssSafeguard20b,
			// TODO: Figure out a good value for this.
			// We don't use the reasoning channel at all, but higher reasoning produces better results (at least as far as I can tell)
			ReasoningEffort: openai.ReasoningEffortNone,
			Messages: []openai.ChatCompletionMessageParamUnion{
				{
					OfSystem: &openai.ChatCompletionSystemMessageParam{
						Role: "system",
						Content: openai.ChatCompletionSystemMessageParamContentUnion{
							OfString: openai.String(strings.TrimSpace(SafeguardDefaultSystemPrompt)),
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
			if violation.Violation {
				// TODO: Return further classifications depending on `violation.PolicyNum`
				return []classification.Classification{classification.Spam}, nil
			}
		}
	}
	return nil, nil
}
