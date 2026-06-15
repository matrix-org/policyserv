package ai

import (
	"context"
	"testing"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/test"
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
	mockApi := test.MakeOpenAIModerationServer(t, apiKey)
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

	spammyEvent1 := test.MustMakeKeywordEvent(test.KeywordSpammy)
	ret, err := provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: true}, &Input{Event: spammyEvent1})
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.ProhibitedContent(harms.SpamGeneral), ret)

	spammyEvent2 := test.MustMakeKeywordEvent(test.KeywordSpammyCSAM)
	ret, err = provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: true}, &Input{Event: spammyEvent2})
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.ProhibitedContent(
		harms.SpamGeneral,
		harms.ChildSafetyCSAM, // should have been detected
	), ret)

	neutralEvent := test.MustMakeKeywordEvent(test.KeywordNeutral)
	ret, err = provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: true}, &Input{Event: neutralEvent})
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.NeutralContent(), ret) // no classifications

	failEvent := test.MustMakeKeywordEvent(test.KeywordIntentionalFail)

	// First test that when FailSecure: true we return a spam classification
	ret, err = provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: true}, &Input{Event: failEvent})
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.ProhibitedContent(
		harms.OtherGeneral, // added by the provider
	), ret)

	// Now when FailSecure: false, we should get no classifications (but also no errors)
	ret, err = provider.CheckEvent(context.Background(), &OpenAIOmniModerationConfig{FailSecure: false}, &Input{Event: failEvent})
	assert.NoError(t, err)
	test.AssertEqualContentInfo(t, harms.NeutralContent(), ret)
}
