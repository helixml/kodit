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
	// Env: DATA_DIR
	// Default: ~/.kodit
	DataDir string `envconfig:"DATA_DIR"`

	// DBURL is the database connection URL.
	// Env: DB_URL
	// Default: sqlite:///{data_dir}/kodit.db
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

	// PeriodicSync configures periodic repository syncing.
	PeriodicSync PeriodicSyncEnv `envconfig:"PERIODIC_SYNC"`

	// Remote configures remote server connection.
	Remote RemoteEnv `envconfig:"REMOTE"`

	// Reporting configures progress reporting.
	Reporting ReportingEnv `envconfig:"REPORTING"`

	// LiteLLMCache configures LLM response caching.
	LiteLLMCache LiteLLMCacheEnv `envconfig:"LITELLM_CACHE"`

	// WorkerCount is the number of background workers.
	// Env: WORKER_COUNT (default: 1)
	WorkerCount int `envconfig:"WORKER_COUNT" default:"1"`

	// SearchLimit is the default search result limit.
	// Env: SEARCH_LIMIT (default: 10)
	SearchLimit int `envconfig:"SEARCH_LIMIT" default:"10"`

	// HTTPCacheDir is the directory for caching HTTP responses to disk.
	// When set, POST request/response pairs are cached to avoid repeated API calls.
	// Env: HTTP_CACHE_DIR
	HTTPCacheDir string `envconfig:"HTTP_CACHE_DIR"`
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
	// Env: *_NUM_PARALLEL_TASKS (default: 1)
	NumParallelTasks int `envconfig:"NUM_PARALLEL_TASKS" default:"1"`

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

	// MaxBatchChars is the maximum total characters per embedding batch.
	// Env: *_MAX_BATCH_CHARS (default: 16000)
	MaxBatchChars int `envconfig:"MAX_BATCH_CHARS" default:"16000"`

	// MaxBatchSize is the maximum number of requests per batch.
	// Env: *_MAX_BATCH_SIZE (default: 1)
	MaxBatchSize int `envconfig:"MAX_BATCH_SIZE" default:"1"`
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

	// Worker count
	if e.WorkerCount > 0 {
		cfg = applyOption(cfg, WithWorkerCount(e.WorkerCount))
	}

	// Search limit
	if e.SearchLimit > 0 {
		cfg = applyOption(cfg, WithSearchLimit(e.SearchLimit))
	}

	// HTTP cache directory
	if e.HTTPCacheDir != "" {
		cfg = applyOption(cfg, WithHTTPCacheDir(e.HTTPCacheDir))
	}

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
		WithMaxBatchChars(e.MaxBatchChars),
		WithMaxBatchSize(e.MaxBatchSize),
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
