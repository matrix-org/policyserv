package config

import (
	"database/sql/driver"
	"encoding/json"
)

type CommunityConfig struct {
	// Note: The `json` tag does *not* use `default`, but `envconfig` will. This allows us to have 3
	// levels of config: default, process/instance, and JSON (for communities).
	//
	// Note: `omitempty` is used to avoid filling the database with null/zero values. We use pointer types to ensure
	// that communities can set "negative" values like `sticky_events_filter_allow_sticky_events: false` though (otherwise
	// the community would be stuck with the envconfig/instance default).

	KeywordFilterKeywords                    *[]string `json:"keyword_filter_keywords,omitempty" envconfig:"keyword_filter_keywords" default:"spammy spam"`
	KeywordTemplateFilterTemplateNames       *[]string `json:"keyword_template_filter_template_names,omitempty" envconfig:"keyword_template_filter_template_names" default:""`
	MentionFilterMaxMentions                 *int      `json:"mention_filter_max_mentions,omitempty" envconfig:"mention_filter_max_mentions" default:"20"`
	MentionFilterMinPlaintextLength          *int      `json:"mention_filter_min_plaintext_length,omitempty" envconfig:"mention_filter_min_plaintext_length" default:"5"`
	ManyAtsFilterMaxAts                      *int      `json:"many_ats_filter_max_ats,omitempty" envconfig:"many_ats_filter_max_ats" default:"20"`
	MediaFilterMediaTypes                    *[]string `json:"media_filter_media_types,omitempty" envconfig:"media_filter_media_types" default:"m.sticker,m.image,m.video,m.file,m.audio"`
	UntrustedMediaFilterMediaTypes           *[]string `json:"untrusted_media_filter_media_types,omitempty" envconfig:"untrusted_media_filter_media_types" default:"m.sticker,m.image,m.video,m.file,m.audio"`
	UntrustedMediaFilterUseMuninn            *bool     `json:"untrusted_media_filter_use_muninn,omitempty" envconfig:"untrusted_media_filter_use_muninn" default:"true"`
	UntrustedMediaFilterUsePowerLevels       *bool     `json:"untrusted_media_filter_use_power_levels,omitempty" envconfig:"untrusted_media_filter_use_power_levels" default:"true"`
	UntrustedMediaFilterAllowedUserGlobs     *[]string `json:"untrusted_media_filter_allowed_user_globs,omitempty" envconfig:"untrusted_media_filter_allowed_user_globs" default:""`
	UntrustedMediaFilterDeniedUserGlobs      *[]string `json:"untrusted_media_filter_denied_user_globs,omitempty" envconfig:"untrusted_media_filter_denied_user_globs" default:""`
	DensityFilterMaxDensity                  *float64  `json:"density_filter_max_density,omitempty" envconfig:"density_filter_max_density" default:"0.95"`
	DensityFilterMinTriggerLength            *int      `json:"density_filter_min_trigger_length,omitempty" envconfig:"density_filter_min_trigger_length" default:"150"`
	TrimLengthFilterMaxDifference            *int      `json:"trim_length_filter_max_difference,omitempty" envconfig:"trim_length_filter_max_difference" default:"25"`
	LengthFilterMaxLength                    *int      `json:"length_filter_max_length,omitempty" envconfig:"length_filter_max_length" default:"10000"`
	SenderPrefilterAllowedSenders            *[]string `json:"sender_prefilter_allowed_senders,omitempty" envconfig:"sender_prefilter_allowed_senders" default:""`
	EventTypePrefilterAllowedEventTypes      *[]string `json:"event_type_prefilter_allowed_event_types,omitempty" envconfig:"event_type_prefilter_allowed_event_types" default:"m.room.redaction"`
	EventTypePrefilterAllowedStateEventTypes *[]string `json:"event_type_prefilter_allowed_state_event_types,omitempty" envconfig:"event_type_prefilter_allowed_state_event_types" default:"m.room.power_levels,m.room.avatar,m.room.name,m.room.topic,m.room.join_rules,m.room.history_visibility,m.room.create,m.room.server_acl,m.room.tombstone,m.room.encryption,m.room.canonical_alias"`
	HellbanPostfilterMinutes                 *int      `json:"hellban_postfilter_minutes,omitempty" envconfig:"hellban_postfilter_minutes" default:"60"`
	MjolnirFilterEnabled                     *bool     `json:"mjolnir_filter_enabled,omitempty" envconfig:"mjolnir_filter_enabled" default:"true"`
	SpamThreshold                            *float64  `json:"spam_threshold,omitempty" envconfig:"spam_threshold" default:"0.8"`
	WebhookUrl                               *string   `json:"webhook_url,omitempty" envconfig:"webhook_url" default:""`
	OpenAIFilterFailSecure                   *bool     `json:"openai_filter_fail_secure,omitempty" envconfig:"openai_filter_fail_secure" default:"true"`
	GptOssSafeguardFilterFailSecure          *bool     `json:"gpt_oss_safeguard_filter_fail_secure,omitempty" envconfig:"gpt_oss_safeguard_filter_fail_secure" default:"true"`
	StickyEventsFilterAllowStickyEvents      *bool     `json:"sticky_events_filter_allow_sticky_events,omitempty" envconfig:"sticky_events_filter_allow_sticky_events" default:"true"`
	HMAFilterEnabledBanks                    *[]string `json:"hma_filter_enabled_banks,omitempty" envconfig:"hma_filter_enabled_banks" default:""`
}

func (c *CommunityConfig) Clone() (*CommunityConfig, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	clone := &CommunityConfig{}
	err = json.Unmarshal(b, &clone)
	if err != nil {
		return nil, err
	}
	return clone, nil
}

// --- SQL driver support ---

func (c *CommunityConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *CommunityConfig) Scan(src interface{}) error {
	b, ok := src.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, &c)
}
