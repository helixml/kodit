// Package config provides application configuration.
package config

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// EnvConfig holds all environment-based configuration.
// Field names map to environment variables with KODIT_ prefix removed.
// Nested structs use underscore delimiter (e.g., EMBEDDING_ENDPOINT_BASE_URL).
type EnvConfig struct {
	// Host is the server host to bind to.
	// Env: HOST (default: 0.0.0.0)
	Host string `envconfig:"HOST" default:"0.0.0.0"`

	// Port is the server port to listen on.
	// Env: PORT (default: 8080)
	Port int `envconfig:"PORT" default:"8080"`

	// DataDir is the data directory path.
	// Env: DATA_DIR (default: .kodit)
	DataDir string `envconfig:"DATA_DIR"`

	// DBURL is the database connection URL.
	// Env: DB_URL (default: sqlite:///{data_dir}/kodit.db)
	DBURL string `envconfig:"DB_URL"`

	// LogLevel is the log verbosity level.
	// Env: LOG_LEVEL (default: INFO)
	LogLevel string `envconfig:"LOG_LEVEL" default:"INFO"`

	// LogFormat is the log output format (pretty or json).
	// Env: LOG_FORMAT (default: pretty)
	LogFormat string `envconfig:"LOG_FORMAT" default:"pretty"`

	// DisableTelemetry controls telemetry collection.
	// Env: DISABLE_TELEMETRY (default: false)
	DisableTelemetry bool `envconfig:"DISABLE_TELEMETRY" default:"false"`

	// SkipProviderValidation skips provider requirement validation at startup.
	// Env: SKIP_PROVIDER_VALIDATION (default: false)
	// WARNING: For testing only. Kodit requires providers for full functionality.
	SkipProviderValidation bool `envconfig:"SKIP_PROVIDER_VALIDATION" default:"false"`

	// APIKeys is a comma-separated list of valid API keys.
	// Env: API_KEYS
	APIKeys string `envconfig:"API_KEYS"`

	// EmbeddingEndpoint configures the embedding AI service.
	EmbeddingEndpoint EndpointEnv `envconfig:"EMBEDDING_ENDPOINT"`

	// EnrichmentEndpoint configures the enrichment AI service.
	EnrichmentEndpoint EndpointEnv `envconfig:"ENRICHMENT_ENDPOINT"`

	// Search configures the search backend.
	Search SearchEnv `envconfig:"DEFAULT_SEARCH"`

	// Git configures the Git provider.
	Git GitEnv `envconfig:"GIT"`

	// PeriodicSync configures periodic repository syncing.
	PeriodicSync PeriodicSyncEnv `envconfig:"PERIODIC_SYNC"`

	// Remote configures remote server connection.
	Remote RemoteEnv `envconfig:"REMOTE"`

	// Reporting configures progress reporting.
	Reporting ReportingEnv `envconfig:"REPORTING"`

	// LiteLLMCache configures LLM response caching.
	LiteLLMCache LiteLLMCacheEnv `envconfig:"LITELLM_CACHE"`
}

// EndpointEnv holds environment configuration for an AI endpoint.
type EndpointEnv struct {
	// BaseURL is the base URL for the endpoint.
	// Env: *_BASE_URL
	BaseURL string `envconfig:"BASE_URL"`

	// Model is the model identifier (e.g., openai/text-embedding-3-small).
	// Env: *_MODEL
	Model string `envconfig:"MODEL"`

	// APIKey is the API key for authentication.
	// Env: *_API_KEY
	APIKey string `envconfig:"API_KEY"`

	// NumParallelTasks is the number of parallel tasks.
	// Env: *_NUM_PARALLEL_TASKS (default: 10)
	NumParallelTasks int `envconfig:"NUM_PARALLEL_TASKS" default:"10"`

	// SocketPath is the Unix socket path for local communication.
	// Env: *_SOCKET_PATH
	SocketPath string `envconfig:"SOCKET_PATH"`

	// Timeout is the request timeout in seconds.
	// Env: *_TIMEOUT (default: 60)
	Timeout float64 `envconfig:"TIMEOUT" default:"60"`

	// MaxRetries is the maximum number of retries.
	// Env: *_MAX_RETRIES (default: 5)
	MaxRetries int `envconfig:"MAX_RETRIES" default:"5"`

	// InitialDelay is the initial retry delay in seconds.
	// Env: *_INITIAL_DELAY (default: 2.0)
	InitialDelay float64 `envconfig:"INITIAL_DELAY" default:"2.0"`

	// BackoffFactor is the retry backoff multiplier.
	// Env: *_BACKOFF_FACTOR (default: 2.0)
	BackoffFactor float64 `envconfig:"BACKOFF_FACTOR" default:"2.0"`

	// ExtraParams is a JSON-encoded map of extra parameters.
	// Env: *_EXTRA_PARAMS
	ExtraParams string `envconfig:"EXTRA_PARAMS"`

	// MaxTokens is the maximum token limit.
	// Env: *_MAX_TOKENS (default: 4000)
	MaxTokens int `envconfig:"MAX_TOKENS" default:"4000"`
}

// SearchEnv holds environment configuration for search.
type SearchEnv struct {
	// Provider is the search provider (sqlite or vectorchord).
	// Env: DEFAULT_SEARCH_PROVIDER (default: sqlite)
	Provider string `envconfig:"PROVIDER" default:"sqlite"`
}

// GitEnv holds environment configuration for Git.
type GitEnv struct {
	// Provider is the Git provider (pygit2, gitpython, or dulwich).
	// Env: GIT_PROVIDER (default: dulwich)
	Provider string `envconfig:"PROVIDER" default:"dulwich"`
}

// PeriodicSyncEnv holds environment configuration for periodic sync.
type PeriodicSyncEnv struct {
	// Enabled controls whether periodic sync is enabled.
	// Env: PERIODIC_SYNC_ENABLED (default: true)
	Enabled bool `envconfig:"ENABLED" default:"true"`

	// IntervalSeconds is the sync interval in seconds.
	// Env: PERIODIC_SYNC_INTERVAL_SECONDS (default: 1800)
	IntervalSeconds float64 `envconfig:"INTERVAL_SECONDS" default:"1800"`

	// RetryAttempts is the number of retry attempts.
	// Env: PERIODIC_SYNC_RETRY_ATTEMPTS (default: 3)
	RetryAttempts int `envconfig:"RETRY_ATTEMPTS" default:"3"`
}

// RemoteEnv holds environment configuration for remote server.
type RemoteEnv struct {
	// ServerURL is the remote server URL.
	// Env: REMOTE_SERVER_URL
	ServerURL string `envconfig:"SERVER_URL"`

	// APIKey is the API key for authentication.
	// Env: REMOTE_API_KEY
	APIKey string `envconfig:"API_KEY"`

	// Timeout is the request timeout in seconds.
	// Env: REMOTE_TIMEOUT (default: 30)
	Timeout float64 `envconfig:"TIMEOUT" default:"30"`

	// MaxRetries is the maximum retry attempts.
	// Env: REMOTE_MAX_RETRIES (default: 3)
	MaxRetries int `envconfig:"MAX_RETRIES" default:"3"`

	// VerifySSL controls SSL certificate verification.
	// Env: REMOTE_VERIFY_SSL (default: true)
	VerifySSL bool `envconfig:"VERIFY_SSL" default:"true"`
}

// ReportingEnv holds environment configuration for reporting.
type ReportingEnv struct {
	// LogTimeInterval is the logging interval in seconds.
	// Env: REPORTING_LOG_TIME_INTERVAL (default: 5)
	LogTimeInterval float64 `envconfig:"LOG_TIME_INTERVAL" default:"5"`
}

// LiteLLMCacheEnv holds environment configuration for LLM caching.
type LiteLLMCacheEnv struct {
	// Enabled controls whether caching is enabled.
	// Env: LITELLM_CACHE_ENABLED (default: true)
	Enabled bool `envconfig:"ENABLED" default:"true"`
}

// LoadFromEnv loads configuration from environment variables.
// It uses no prefix, matching the Python pydantic-settings behavior.
func LoadFromEnv() (EnvConfig, error) {
	var cfg EnvConfig
	if err := envconfig.Process("", &cfg); err != nil {
		return EnvConfig{}, err
	}
	return cfg, nil
}

// LoadFromEnvWithPrefix loads configuration with a custom prefix.
// For example, prefix "KODIT" would require KODIT_DATA_DIR instead of DATA_DIR.
func LoadFromEnvWithPrefix(prefix string) (EnvConfig, error) {
	var cfg EnvConfig
	if err := envconfig.Process(prefix, &cfg); err != nil {
		return EnvConfig{}, err
	}
	return cfg, nil
}

// ToAppConfig converts EnvConfig to AppConfig.
func (e EnvConfig) ToAppConfig() AppConfig {
	cfg := NewAppConfig()

	// Apply overrides from environment
	if e.Host != "" {
		cfg = applyOption(cfg, WithHost(e.Host))
	}
	if e.Port != 0 {
		cfg = applyOption(cfg, WithPort(e.Port))
	}
	if e.DataDir != "" {
		cfg = applyOption(cfg, WithDataDir(e.DataDir))
	}
	if e.DBURL != "" {
		cfg = applyOption(cfg, WithDBURL(e.DBURL))
	}
	if e.LogLevel != "" {
		cfg = applyOption(cfg, WithLogLevel(e.LogLevel))
	}
	if e.LogFormat != "" {
		cfg = applyOption(cfg, WithLogFormat(parseLogFormat(e.LogFormat)))
	}
	cfg = applyOption(cfg, WithDisableTelemetry(e.DisableTelemetry))
	cfg = applyOption(cfg, WithSkipProviderValidation(e.SkipProviderValidation))

	if e.APIKeys != "" {
		cfg = applyOption(cfg, WithAPIKeys(ParseAPIKeys(e.APIKeys)))
	}

	// Embedding endpoint
	if e.EmbeddingEndpoint.IsConfigured() {
		cfg = applyOption(cfg, WithEmbeddingEndpoint(e.EmbeddingEndpoint.ToEndpoint()))
	}

	// Enrichment endpoint
	if e.EnrichmentEndpoint.IsConfigured() {
		cfg = applyOption(cfg, WithEnrichmentEndpoint(e.EnrichmentEndpoint.ToEndpoint()))
	}

	// Search config
	cfg = applyOption(cfg, WithSearchConfig(e.Search.ToSearchConfig()))

	// Git config
	cfg = applyOption(cfg, WithGitConfig(e.Git.ToGitConfig()))

	// Periodic sync config
	cfg = applyOption(cfg, WithPeriodicSyncConfig(e.PeriodicSync.ToPeriodicSyncConfig()))

	// Remote config
	if e.Remote.IsConfigured() {
		cfg = applyOption(cfg, WithRemoteConfig(e.Remote.ToRemoteConfig()))
	}

	// Reporting config
	cfg = applyOption(cfg, WithReportingConfig(e.Reporting.ToReportingConfig()))

	// LiteLLM cache config
	cfg = applyOption(cfg, WithLiteLLMCacheConfig(e.LiteLLMCache.ToLiteLLMCacheConfig()))

	return cfg
}

// applyOption applies an option to the config.
func applyOption(cfg AppConfig, opt AppConfigOption) AppConfig {
	opt(&cfg)
	return cfg
}

// IsConfigured returns true if the endpoint has a model configured.
func (e EndpointEnv) IsConfigured() bool {
	return e.Model != ""
}

// ToEndpoint converts EndpointEnv to Endpoint.
func (e EndpointEnv) ToEndpoint() Endpoint {
	opts := []EndpointOption{
		WithModel(e.Model),
		WithNumParallelTasks(e.NumParallelTasks),
		WithTimeout(time.Duration(e.Timeout * float64(time.Second))),
		WithMaxRetries(e.MaxRetries),
		WithInitialDelay(time.Duration(e.InitialDelay * float64(time.Second))),
		WithBackoffFactor(e.BackoffFactor),
		WithMaxTokens(e.MaxTokens),
	}

	if e.BaseURL != "" {
		opts = append(opts, WithBaseURL(e.BaseURL))
	}
	if e.APIKey != "" {
		opts = append(opts, WithAPIKey(e.APIKey))
	}
	if e.SocketPath != "" {
		opts = append(opts, WithSocketPath(e.SocketPath))
	}
	if e.ExtraParams != "" {
		params := parseExtraParams(e.ExtraParams)
		if params != nil {
			opts = append(opts, WithExtraParams(params))
		}
	}

	return NewEndpointWithOptions(opts...)
}

// ToSearchConfig converts SearchEnv to SearchConfig.
func (s SearchEnv) ToSearchConfig() SearchConfig {
	cfg := NewSearchConfig()
	switch strings.ToLower(s.Provider) {
	case "vectorchord":
		return cfg.WithProvider(SearchProviderVectorChord)
	default:
		return cfg.WithProvider(SearchProviderSQLite)
	}
}

// ToGitConfig converts GitEnv to GitConfig.
func (g GitEnv) ToGitConfig() GitConfig {
	cfg := NewGitConfig()
	switch strings.ToLower(g.Provider) {
	case "pygit2":
		return cfg.WithProvider(GitProviderPygit2)
	case "gitpython":
		return cfg.WithProvider(GitProviderGitPython)
	default:
		return cfg.WithProvider(GitProviderDulwich)
	}
}

// ToPeriodicSyncConfig converts PeriodicSyncEnv to PeriodicSyncConfig.
func (p PeriodicSyncEnv) ToPeriodicSyncConfig() PeriodicSyncConfig {
	return NewPeriodicSyncConfig().
		WithEnabled(p.Enabled).
		WithIntervalSeconds(p.IntervalSeconds).
		WithRetryAttempts(p.RetryAttempts)
}

// IsConfigured returns true if remote server URL is configured.
func (r RemoteEnv) IsConfigured() bool {
	return r.ServerURL != ""
}

// ToRemoteConfig converts RemoteEnv to RemoteConfig.
func (r RemoteEnv) ToRemoteConfig() RemoteConfig {
	opts := []RemoteConfigOption{
		WithRemoteTimeout(time.Duration(r.Timeout * float64(time.Second))),
		WithRemoteMaxRetries(r.MaxRetries),
		WithVerifySSL(r.VerifySSL),
	}

	if r.ServerURL != "" {
		opts = append(opts, WithServerURL(r.ServerURL))
	}
	if r.APIKey != "" {
		opts = append(opts, WithRemoteAPIKey(r.APIKey))
	}

	return NewRemoteConfigWithOptions(opts...)
}

// ToReportingConfig converts ReportingEnv to ReportingConfig.
func (r ReportingEnv) ToReportingConfig() ReportingConfig {
	return NewReportingConfig().
		WithLogTimeInterval(time.Duration(r.LogTimeInterval * float64(time.Second)))
}

// ToLiteLLMCacheConfig converts LiteLLMCacheEnv to LiteLLMCacheConfig.
func (l LiteLLMCacheEnv) ToLiteLLMCacheConfig() LiteLLMCacheConfig {
	return NewLiteLLMCacheConfig().WithEnabled(l.Enabled)
}

// parseLogFormat parses a log format string.
func parseLogFormat(s string) LogFormat {
	switch strings.ToLower(s) {
	case "json":
		return LogFormatJSON
	default:
		return LogFormatPretty
	}
}

// parseExtraParams parses JSON-encoded extra parameters.
func parseExtraParams(s string) map[string]any {
	if s == "" {
		return nil
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(s), &params); err != nil {
		return nil
	}
	return params
}
