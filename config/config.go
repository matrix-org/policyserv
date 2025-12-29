package config

import (
	"github.com/kelseyhightower/envconfig"
)

type InstanceConfig struct {
	HttpBind    string `envconfig:"http_bind" default:"0.0.0.0:8080"`
	MetricsBind string `envconfig:"http_metrics_bind" default:"0.0.0.0:8081"`
	PprofBind   string `envconfig:"http_pprof_bind" default:""`

	ApiKey string `envconfig:"api_key" default:""`

	Database                string   `envconfig:"database" default:"postgres://policyserv:devonly@localhost/policyserv?sslmode=disable"`
	DatabaseMigrationsDir   string   `envconfig:"database_migrations_dir" default:"./migrations"`
	DatabaseMaxOpenConns    int      `envconfig:"database_max_open_conns" default:"10"`
	DatabaseMaxIdleConns    int      `envconfig:"database_max_idle_conns" default:"5"`
	DatabaseReadonlyUri     string   `envconfig:"database_read" default:""`
	DatabaseReadonlyMaxOpen int      `envconfig:"database_read_max_open" default:"10"`
	DatabaseReadonlyMaxIdle int      `envconfig:"database_read_max_idle" default:"5"`
	ProcessingPoolSize      int      `envconfig:"processing_pool_size" default:"100"`
	KeyQueryServer          []string `envconfig:"key_query_server" default:"matrix.org,ed25519:a_RXGa,l8Hft5qXKn1vfHrg3p4+W8gELQVo8N13JkluMfmn2sQ"`
	EnableDirectKeyFetching bool     `envconfig:"direct_key_fetching" default:"false"`
	TrustedOrigins          []string `envconfig:"trusted_origins" default:"matrix.org,element.io"`
	StateCacheMinutes       int      `envconfig:"state_cache_minutes" default:"5"`

	HomeserverName                   string `envconfig:"homeserver_name" default:"localhost"`
	HomeserverSigningKeyPath         string `envconfig:"homeserver_signing_key_path" default:"./signing.key"`
	HomeserverEventSigningKeyPath    string `envconfig:"homeserver_event_signing_key_path" default:"./signing.key"`
	HomeserverMediaClientUrl         string `envconfig:"homeserver_media_client_url" default:"https://matrix-client.matrix.org"`
	HomeserverMediaClientAccessToken string `envconfig:"homeserver_media_client_access_token" default:""`

	// Note: the Mjolnir filter can't be configured by communities at the moment
	MjolnirFilterRoomID string `envconfig:"mjolnir_filter_room_id" default:""`

	// Note: the OpenAI filter can't be configured by communities at the moment
	OpenAIApiKey         string   `envconfig:"openai_filter_api_key" default:""`
	OpenAIAllowedRoomIds []string `envconfig:"openai_filter_allowed_room_ids" default:""`

	MuninnHallSourceApiUrl string `envconfig:"muninn_hall_source_api_url" default:"https://mau.bot/_matrix/maubot/plugin/muninnbot/member_directory"`
	MuninnHallSourceApiKey string `envconfig:"muninn_hall_source_api_key" default:""`

	JoinRoomIDs   []string `envconfig:"join_room_ids" default:""`
	JoinServer    string   `envconfig:"join_server" default:"matrix.org"`
	JoinLocalpart string   `envconfig:"join_localpart" default:"policyserv"`

	// [Client-Server API domain (no scheme)]:[access token]
	// Example: matrix-client.matrix.org:syt_example
	ModeratorAccessTokens map[string]string `envconfig:"moderator_access_tokens" default:""`
	ModerationPoolSize    int               `envconfig:"moderation_pool_size" default:"25"`

	AllowedWebhookDomains []string `envconfig:"allowed_webhook_domains" default:"element.ems.host"`
	WebhookPoolSize       int      `envconfig:"webhook_pool_size" default:"5"`

	HMAApiUrl string `envconfig:"hma_api_url" default:""`
	HMAApiKey string `envconfig:"hma_api_key" default:""`

	GptOssSafeguardModelName       string                         `envconfig:"gpt_oss_safeguard_model_name" default:"openai/gpt-oss-safeguard-120b"`
	GptOssSafeguardOpenAIApiUrl    string                         `envconfig:"gpt_oss_safeguard_openai_api_url" default:"http://localhost:1234/v1/"`
	GptOssSafeguardAllowedRoomIds  []string                       `envconfig:"gpt_oss_safeguard_allowed_room_ids" default:""`
	GptOssSafeguardReasoningEffort GptOssSafeguardReasoningEffort `envconfig:"gpt_oss_safeguard_reasoning_effort" default:"low"`
}

func NewInstanceConfig() (*InstanceConfig, error) {
	cnf := &InstanceConfig{}
	err := envconfig.Process("ps", cnf)
	return cnf, err
}
