package config

import (
	"testing"

	"github.com/matrix-org/policyserv/internal"
	"github.com/stretchr/testify/assert"
)

func TestNewCommunityConfigForJSON(t *testing.T) {
	// NewCommunityConfigForJSON calls newCommunityConfigForJSONWithBase internally

	testBase := &CommunityConfig{
		InlineEmojiSizeFilterMaxHeightPixels: internal.Pointer(100),
		KeywordFilterKeywords:                &[]string{"spammy spam", "example"},
	}
	testJSON := []byte(`{
		"inline_emoji_size_filter_max_height_pixels": 36,
		"length_filter_max_length": 12
	}`)
	testConfig, err := newCommunityConfigForJSONWithBase(testBase, testJSON)
	assert.NoError(t, err)
	assert.Equal(t, 36, *testConfig.InlineEmojiSizeFilterMaxHeightPixels)
	assert.Equal(t, 12, *testConfig.LengthFilterMaxLength)
	assert.Equal(t, []string{"spammy spam", "example"}, *testConfig.KeywordFilterKeywords)
}

func TestEnvconfigAssumptions(t *testing.T) {
	// Here we want to ensure that defaults from envconfig are set correctly, and that the JSON unmarshalling
	// doesn't use the same defaults. If both parsers use the defaults, that would mean all community configs
	// will use defaults instead of "instance config".

	t.Cleanup(func() {
		t.Setenv("PS_INLINE_EMOJI_SIZE_FILTER_MAX_HEIGHT_PIXELS", "")
		t.Setenv("PS_KEYWORD_FILTER_KEYWORDS", "")
	})
	t.Setenv("PS_INLINE_EMOJI_SIZE_FILTER_MAX_HEIGHT_PIXELS", "100")
	t.Setenv("PS_KEYWORD_FILTER_KEYWORDS", "spammy spam,example")

	buildBaseConfig()
	assert.Equal(t, 100, *baseConfigRaw.InlineEmojiSizeFilterMaxHeightPixels)
	assert.Equal(t, []string{"spammy spam", "example"}, *baseConfigRaw.KeywordFilterKeywords)
	assert.Equal(t, 10000, *baseConfigRaw.LengthFilterMaxLength) // default
	assert.Equal(t, 20, *baseConfigRaw.ManyAtsFilterMaxAts)      // default

	testJSON := []byte(`{
		"inline_emoji_size_filter_max_height_pixels": 36,
		"length_filter_max_length": 12
	}`)
	testConfig, err := newCommunityConfigForJSONWithBase(baseConfigRaw, testJSON)
	assert.NoError(t, err)

	// JSON overrides
	assert.Equal(t, 36, *testConfig.InlineEmojiSizeFilterMaxHeightPixels)
	assert.Equal(t, 12, *testConfig.LengthFilterMaxLength)

	// Base config overrides
	assert.Equal(t, []string{"spammy spam", "example"}, *testConfig.KeywordFilterKeywords)

	// Default values
	assert.Equal(t, 20, *testConfig.ManyAtsFilterMaxAts)
}

func TestLayeringNegation(t *testing.T) {
	// Verify that communities can set zero values to turn off instance-configured values.
	// For example, being able to turn off sticky events if they were enabled by the instance.

	t.Cleanup(func() {
		t.Setenv("PS_STICKY_EVENTS_FILTER_ALLOW_STICKY_EVENTS", "")
	})
	t.Setenv("PS_STICKY_EVENTS_FILTER_ALLOW_STICKY_EVENTS", "true")

	buildBaseConfig()
	assert.Equal(t, true, *baseConfigRaw.StickyEventsFilterAllowStickyEvents)

	testJSON := []byte(`{
		"sticky_events_filter_allow_sticky_events": false
	}`)
	testConfig, err := newCommunityConfigForJSONWithBase(baseConfigRaw, testJSON)
	assert.NoError(t, err)
	assert.Equal(t, false, *testConfig.StickyEventsFilterAllowStickyEvents)
}

func TestNewDefaultCommunityConfig(t *testing.T) {
	// When calling the function, we shouldn't be getting any envconfig values

	t.Cleanup(func() {
		t.Setenv("PS_INLINE_EMOJI_SIZE_FILTER_MAX_HEIGHT_PIXELS", "")
		t.Setenv("PS_KEYWORD_FILTER_KEYWORDS", "")
	})
	t.Setenv("PS_INLINE_EMOJI_SIZE_FILTER_MAX_HEIGHT_PIXELS", "100")
	t.Setenv("PS_KEYWORD_FILTER_KEYWORDS", "spammy spam,example")

	cnf, err := NewDefaultCommunityConfig()
	assert.NoError(t, err)
	assert.NotEqual(t, 100, *cnf.InlineEmojiSizeFilterMaxHeightPixels)
	assert.NotEqual(t, []string{"spammy spam", "example"}, *cnf.KeywordFilterKeywords)
}
