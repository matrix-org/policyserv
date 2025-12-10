package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCommunityConfigForJSON(t *testing.T) {
	// NewCommunityConfigForJSON calls newCommunityConfigForJSONWithBase internally

	testBase := &CommunityConfig{
		SpamThreshold:         0.2,
		KeywordFilterKeywords: []string{"spammy spam", "example"},
	}
	testJSON := []byte(`{
		"spam_threshold": 0.5,
		"length_filter_max_length": 12
	}`)
	testConfig, err := newCommunityConfigForJSONWithBase(testBase, testJSON)
	assert.NoError(t, err)
	assert.Equal(t, 0.5, testConfig.SpamThreshold)
	assert.Equal(t, 12, testConfig.LengthFilterMaxLength)
	assert.Equal(t, []string{"spammy spam", "example"}, testConfig.KeywordFilterKeywords)
}

func TestEnvconfigAssumptions(t *testing.T) {
	// Here we want to ensure that defaults from envconfig are set correctly, and that the JSON unmarshalling
	// doesn't use the same defaults. If both parsers use the defaults, that would mean all community configs
	// will use defaults instead of "instance config".

	t.Cleanup(func() {
		t.Setenv("PS_SPAM_THRESHOLD", "")
		t.Setenv("PS_KEYWORD_FILTER_KEYWORDS", "")
	})
	t.Setenv("PS_SPAM_THRESHOLD", "0.2")
	t.Setenv("PS_KEYWORD_FILTER_KEYWORDS", "spammy spam,example")

	buildBaseConfig()
	assert.Equal(t, 0.2, baseConfigRaw.SpamThreshold)
	assert.Equal(t, []string{"spammy spam", "example"}, baseConfigRaw.KeywordFilterKeywords)
	assert.Equal(t, 10000, baseConfigRaw.LengthFilterMaxLength) // default
	assert.Equal(t, 20, baseConfigRaw.ManyAtsFilterMaxAts)      // default

	testJSON := []byte(`{
		"spam_threshold": 0.5,
		"length_filter_max_length": 12
	}`)
	testConfig, err := newCommunityConfigForJSONWithBase(baseConfigRaw, testJSON)
	assert.NoError(t, err)

	// JSON overrides
	assert.Equal(t, 0.5, testConfig.SpamThreshold)
	assert.Equal(t, 12, testConfig.LengthFilterMaxLength)

	// Base config overrides
	assert.Equal(t, []string{"spammy spam", "example"}, testConfig.KeywordFilterKeywords)

	// Default values
	assert.Equal(t, 20, testConfig.ManyAtsFilterMaxAts)
}

func TestNewDefaultCommunityConfig(t *testing.T) {
	// When calling the function, we shouldn't be getting any envconfig values

	t.Cleanup(func() {
		t.Setenv("PS_SPAM_THRESHOLD", "")
		t.Setenv("PS_KEYWORD_FILTER_KEYWORDS", "")
	})
	t.Setenv("PS_SPAM_THRESHOLD", "0.2")
	t.Setenv("PS_KEYWORD_FILTER_KEYWORDS", "spammy spam,example")

	cnf, err := NewDefaultCommunityConfig()
	assert.NoError(t, err)
	assert.NotEqual(t, 0.2, cnf.SpamThreshold)
	assert.NotEqual(t, []string{"spammy spam", "example"}, cnf.KeywordFilterKeywords)
}
