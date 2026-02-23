// Package config provides application configuration.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Default configuration values.
const (
	DefaultHost                      = "0.0.0.0"
	DefaultPort                      = 8080
	DefaultLogLevel                  = "INFO"
	DefaultWorkerCount               = 1
	DefaultSearchLimit               = 10
	DefaultCloneSubdir               = "repos"
	DefaultEndpointParallelTasks     = 1
	DefaultEndpointTimeout           = 60 * time.Second
	DefaultEndpointMaxRetries        = 5
	DefaultEndpointInitialDelay      = 2 * time.Second
	DefaultEndpointBackoffFactor     = 2.0
	DefaultEndpointMaxTokens         = 4000
	DefaultPeriodicSyncInterval      = 1800.0 // seconds
	DefaultPeriodicSyncCheckInterval = 10.0   // seconds
	DefaultPeriodicSyncRetries       = 3
	DefaultEndpointMaxBatchChars      = 16000
	DefaultRemoteTimeout             = 30 * time.Second
	DefaultRemoteMaxRetries          = 3
	DefaultReportingInterval         = 5 * time.Second
)

// LogFormat represents the log output format.
type LogFormat string

// LogFormat values.
const (
	LogFormatPretty LogFormat = "pretty"
	LogFormatJSON   LogFormat = "json"
)

// ReportingConfig configures progress reporting.
type ReportingConfig struct {
	logTimeInterval time.Duration
}

// NewReportingConfig creates a new ReportingConfig with defaults.
func NewReportingConfig() ReportingConfig {
	return ReportingConfig{
		logTimeInterval: DefaultReportingInterval,
	}
}

// LogTimeInterval returns the time interval for logging progress.
func (r ReportingConfig) LogTimeInterval() time.Duration {
	return r.logTimeInterval
}

// WithLogTimeInterval returns a new config with the specified interval.
func (r ReportingConfig) WithLogTimeInterval(d time.Duration) ReportingConfig {
	r.logTimeInterval = d
	return r
}

// LiteLLMCacheConfig configures LLM response caching.
type LiteLLMCacheConfig struct {
	enabled bool
}

// NewLiteLLMCacheConfig creates a new LiteLLMCacheConfig with defaults.
func NewLiteLLMCacheConfig() LiteLLMCacheConfig {
	return LiteLLMCacheConfig{
		enabled: true,
	}
}

// Enabled returns whether caching is enabled.
func (c LiteLLMCacheConfig) Enabled() bool {
	return c.enabled
}

// WithEnabled returns a new config with the specified enabled state.
func (c LiteLLMCacheConfig) WithEnabled(enabled bool) LiteLLMCacheConfig {
	c.enabled = enabled
	return c
}

// Endpoint configures an AI service endpoint.
type Endpoint struct {
	baseURL          string
	model            string
	apiKey           string
	numParallelTasks int
	socketPath       string
	timeout          time.Duration
	maxRetries       int
	initialDelay     time.Duration
	backoffFactor    float64
	extraParams      map[string]any
	maxTokens        int
	maxBatchChars    int
}

// NewEndpoint creates a new Endpoint with defaults.
func NewEndpoint() Endpoint {
	return Endpoint{
		numParallelTasks: DefaultEndpointParallelTasks,
		timeout:          DefaultEndpointTimeout,
		maxRetries:       DefaultEndpointMaxRetries,
		initialDelay:     DefaultEndpointInitialDelay,
		backoffFactor:    DefaultEndpointBackoffFactor,
		maxTokens:        DefaultEndpointMaxTokens,
		maxBatchChars:    DefaultEndpointMaxBatchChars,
	}
}

// BaseURL returns the base URL for the endpoint.
func (e Endpoint) BaseURL() string { return e.baseURL }

// Model returns the model identifier.
func (e Endpoint) Model() string { return e.model }

// APIKey returns the API key.
func (e Endpoint) APIKey() string { return e.apiKey }

// NumParallelTasks returns the number of parallel tasks.
func (e Endpoint) NumParallelTasks() int { return e.numParallelTasks }

// SocketPath returns the Unix socket path.
func (e Endpoint) SocketPath() string { return e.socketPath }

// Timeout returns the request timeout.
func (e Endpoint) Timeout() time.Duration { return e.timeout }

// MaxRetries returns the maximum retry count.
func (e Endpoint) MaxRetries() int { return e.maxRetries }

// InitialDelay returns the initial retry delay.
func (e Endpoint) InitialDelay() time.Duration { return e.initialDelay }

// BackoffFactor returns the retry backoff multiplier.
func (e Endpoint) BackoffFactor() float64 { return e.backoffFactor }

// ExtraParams returns additional provider-specific parameters.
func (e Endpoint) ExtraParams() map[string]any {
	if e.extraParams == nil {
		return nil
	}
	result := make(map[string]any, len(e.extraParams))
	for k, v := range e.extraParams {
		result[k] = v
	}
	return result
}

// MaxTokens returns the maximum token limit.
func (e Endpoint) MaxTokens() int { return e.maxTokens }

// MaxBatchChars returns the maximum total characters per embedding batch.
func (e Endpoint) MaxBatchChars() int { return e.maxBatchChars }

// IsConfigured returns true if the endpoint has required configuration.
func (e Endpoint) IsConfigured() bool {
	return e.model != ""
}

// EndpointOption is a functional option for Endpoint.
type EndpointOption func(*Endpoint)

// WithBaseURL sets the base URL.
func WithBaseURL(url string) EndpointOption {
	return func(e *Endpoint) { e.baseURL = url }
}

// WithModel sets the model.
func WithModel(model string) EndpointOption {
	return func(e *Endpoint) { e.model = model }
}

// WithAPIKey sets the API key.
func WithAPIKey(key string) EndpointOption {
	return func(e *Endpoint) { e.apiKey = key }
}

// WithNumParallelTasks sets the parallel task count.
func WithNumParallelTasks(n int) EndpointOption {
	return func(e *Endpoint) { e.numParallelTasks = n }
}

// WithSocketPath sets the Unix socket path.
func WithSocketPath(path string) EndpointOption {
	return func(e *Endpoint) { e.socketPath = path }
}

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) EndpointOption {
	return func(e *Endpoint) { e.timeout = d }
}

// WithMaxRetries sets the maximum retry count.
func WithMaxRetries(n int) EndpointOption {
	return func(e *Endpoint) { e.maxRetries = n }
}

// WithInitialDelay sets the initial retry delay.
func WithInitialDelay(d time.Duration) EndpointOption {
	return func(e *Endpoint) { e.initialDelay = d }
}

// WithBackoffFactor sets the retry backoff multiplier.
func WithBackoffFactor(f float64) EndpointOption {
	return func(e *Endpoint) { e.backoffFactor = f }
}

// WithExtraParams sets extra provider parameters.
func WithExtraParams(params map[string]any) EndpointOption {
	return func(e *Endpoint) {
		if params != nil {
			e.extraParams = make(map[string]any, len(params))
			for k, v := range params {
				e.extraParams[k] = v
			}
		}
	}
}

// WithMaxTokens sets the maximum token limit.
func WithMaxTokens(n int) EndpointOption {
	return func(e *Endpoint) { e.maxTokens = n }
}

// WithMaxBatchChars sets the maximum total characters per embedding batch.
func WithMaxBatchChars(n int) EndpointOption {
	return func(e *Endpoint) { e.maxBatchChars = n }
}

// NewEndpointWithOptions creates an Endpoint with functional options.
func NewEndpointWithOptions(opts ...EndpointOption) Endpoint {
	e := NewEndpoint()
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// PeriodicSyncConfig configures periodic repository syncing.
type PeriodicSyncConfig struct {
	enabled              bool
	intervalSeconds      float64
	checkIntervalSeconds float64
	retryAttempts        int
}

// NewPeriodicSyncConfig creates a new PeriodicSyncConfig with defaults.
func NewPeriodicSyncConfig() PeriodicSyncConfig {
	return PeriodicSyncConfig{
		enabled:              true,
		intervalSeconds:      DefaultPeriodicSyncInterval,
		checkIntervalSeconds: DefaultPeriodicSyncCheckInterval,
		retryAttempts:        DefaultPeriodicSyncRetries,
	}
}

// Enabled returns whether periodic sync is enabled.
func (p PeriodicSyncConfig) Enabled() bool { return p.enabled }

// Interval returns the sync interval as a duration.
func (p PeriodicSyncConfig) Interval() time.Duration {
	return time.Duration(p.intervalSeconds * float64(time.Second))
}

// CheckInterval returns how often to check for repositories due for sync.
func (p PeriodicSyncConfig) CheckInterval() time.Duration {
	return time.Duration(p.checkIntervalSeconds * float64(time.Second))
}

// RetryAttempts returns the retry count.
func (p PeriodicSyncConfig) RetryAttempts() int { return p.retryAttempts }

// WithEnabled returns a new config with the specified enabled state.
func (p PeriodicSyncConfig) WithEnabled(enabled bool) PeriodicSyncConfig {
	p.enabled = enabled
	return p
}

// WithIntervalSeconds returns a new config with the specified interval.
func (p PeriodicSyncConfig) WithIntervalSeconds(seconds float64) PeriodicSyncConfig {
	p.intervalSeconds = seconds
	return p
}

// WithCheckIntervalSeconds returns a new config with the specified check interval.
func (p PeriodicSyncConfig) WithCheckIntervalSeconds(seconds float64) PeriodicSyncConfig {
	p.checkIntervalSeconds = seconds
	return p
}

// WithRetryAttempts returns a new config with the specified retry count.
func (p PeriodicSyncConfig) WithRetryAttempts(attempts int) PeriodicSyncConfig {
	p.retryAttempts = attempts
	return p
}

// RemoteConfig configures remote server connection.
type RemoteConfig struct {
	serverURL  string
	apiKey     string
	timeout    time.Duration
	maxRetries int
	verifySSL  bool
}

// NewRemoteConfig creates a new RemoteConfig with defaults.
func NewRemoteConfig() RemoteConfig {
	return RemoteConfig{
		timeout:    DefaultRemoteTimeout,
		maxRetries: DefaultRemoteMaxRetries,
		verifySSL:  true,
	}
}

// ServerURL returns the remote server URL.
func (r RemoteConfig) ServerURL() string { return r.serverURL }

// APIKey returns the API key.
func (r RemoteConfig) APIKey() string { return r.apiKey }

// Timeout returns the request timeout.
func (r RemoteConfig) Timeout() time.Duration { return r.timeout }

// MaxRetries returns the maximum retry count.
func (r RemoteConfig) MaxRetries() int { return r.maxRetries }

// VerifySSL returns whether SSL verification is enabled.
func (r RemoteConfig) VerifySSL() bool { return r.verifySSL }

// IsConfigured returns true if remote mode is configured.
func (r RemoteConfig) IsConfigured() bool {
	return r.serverURL != ""
}

// RemoteConfigOption is a functional option for RemoteConfig.
type RemoteConfigOption func(*RemoteConfig)

// WithServerURL sets the server URL.
func WithServerURL(url string) RemoteConfigOption {
	return func(r *RemoteConfig) { r.serverURL = url }
}

// WithRemoteAPIKey sets the API key.
func WithRemoteAPIKey(key string) RemoteConfigOption {
	return func(r *RemoteConfig) { r.apiKey = key }
}

// WithRemoteTimeout sets the timeout.
func WithRemoteTimeout(d time.Duration) RemoteConfigOption {
	return func(r *RemoteConfig) { r.timeout = d }
}

// WithRemoteMaxRetries sets the max retries.
func WithRemoteMaxRetries(n int) RemoteConfigOption {
	return func(r *RemoteConfig) { r.maxRetries = n }
}

// WithVerifySSL sets SSL verification.
func WithVerifySSL(verify bool) RemoteConfigOption {
	return func(r *RemoteConfig) { r.verifySSL = verify }
}

// NewRemoteConfigWithOptions creates a RemoteConfig with options.
func NewRemoteConfigWithOptions(opts ...RemoteConfigOption) RemoteConfig {
	r := NewRemoteConfig()
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

// AppConfig holds the main application configuration.
type AppConfig struct {
	host                   string
	port                   int
	dataDir                string
	dbURL                  string
	logLevel               string
	logFormat              LogFormat
	disableTelemetry       bool
	skipProviderValidation bool
	embeddingEndpoint      *Endpoint
	enrichmentEndpoint     *Endpoint
	periodicSync           PeriodicSyncConfig
	apiKeys                []string
	remote                 RemoteConfig
	reporting              ReportingConfig
	litellmCache           LiteLLMCacheConfig
	workerCount            int
	searchLimit            int
}

// DefaultDataDir returns the default data directory.
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".kodit"
	}
	return filepath.Join(home, ".kodit")
}

// DefaultCloneDir returns the default clone directory for a given data directory.
func DefaultCloneDir(dataDir string) string {
	return filepath.Join(dataDir, DefaultCloneSubdir)
}

// DefaultLogger returns the default slog logger for library consumers.
func DefaultLogger() *slog.Logger {
	return slog.Default()
}

// PrepareDataDir creates the data directory if it does not exist and returns it.
func PrepareDataDir(dataDir string) (string, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("create data directory: %w", err)
	}
	return dataDir, nil
}

// PrepareCloneDir resolves the clone directory (defaulting if empty) and creates it.
func PrepareCloneDir(cloneDir, dataDir string) (string, error) {
	if cloneDir == "" {
		cloneDir = DefaultCloneDir(dataDir)
	}
	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		return "", fmt.Errorf("create clone directory: %w", err)
	}
	return cloneDir, nil
}

// NewAppConfig creates a new AppConfig with defaults.
func NewAppConfig() AppConfig {
	dataDir := DefaultDataDir()
	return AppConfig{
		host:             DefaultHost,
		port:             DefaultPort,
		dataDir:          dataDir,
		dbURL:            "sqlite:///" + filepath.Join(dataDir, "kodit.db"),
		logLevel:         DefaultLogLevel,
		logFormat:        LogFormatPretty,
		disableTelemetry: false,
		periodicSync:     NewPeriodicSyncConfig(),
		apiKeys:          []string{},
		remote:           NewRemoteConfig(),
		reporting:        NewReportingConfig(),
		litellmCache:     NewLiteLLMCacheConfig(),
		workerCount:      DefaultWorkerCount,
		searchLimit:      DefaultSearchLimit,
	}
}

// Host returns the server host to bind to.
func (c AppConfig) Host() string { return c.host }

// Port returns the server port to listen on.
func (c AppConfig) Port() int { return c.port }

// Addr returns the combined host:port address.
func (c AppConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.host, c.port)
}

// DataDir returns the data directory path.
func (c AppConfig) DataDir() string { return c.dataDir }

// DBURL returns the database connection URL.
func (c AppConfig) DBURL() string { return c.dbURL }

// LogLevel returns the log level.
func (c AppConfig) LogLevel() string { return c.logLevel }

// LogFormat returns the log format.
func (c AppConfig) LogFormat() LogFormat { return c.logFormat }

// DisableTelemetry returns whether telemetry is disabled.
func (c AppConfig) DisableTelemetry() bool { return c.disableTelemetry }

// SkipProviderValidation returns whether to skip provider validation at startup.
// This is intended for testing only.
func (c AppConfig) SkipProviderValidation() bool { return c.skipProviderValidation }

// EmbeddingEndpoint returns the embedding endpoint config.
func (c AppConfig) EmbeddingEndpoint() *Endpoint { return c.embeddingEndpoint }

// EnrichmentEndpoint returns the enrichment endpoint config.
func (c AppConfig) EnrichmentEndpoint() *Endpoint { return c.enrichmentEndpoint }

// PeriodicSync returns the periodic sync config.
func (c AppConfig) PeriodicSync() PeriodicSyncConfig { return c.periodicSync }

// APIKeys returns the configured API keys.
func (c AppConfig) APIKeys() []string {
	keys := make([]string, len(c.apiKeys))
	copy(keys, c.apiKeys)
	return keys
}

// Remote returns the remote config.
func (c AppConfig) Remote() RemoteConfig { return c.remote }

// Reporting returns the reporting config.
func (c AppConfig) Reporting() ReportingConfig { return c.reporting }

// LiteLLMCache returns the LiteLLM cache config.
func (c AppConfig) LiteLLMCache() LiteLLMCacheConfig { return c.litellmCache }

// WorkerCount returns the number of background workers.
func (c AppConfig) WorkerCount() int { return c.workerCount }

// SearchLimit returns the default search result limit.
func (c AppConfig) SearchLimit() int { return c.searchLimit }

// IsRemote returns true if running in remote mode.
func (c AppConfig) IsRemote() bool {
	return c.remote.IsConfigured()
}

// CloneDir returns the clone directory path.
func (c AppConfig) CloneDir() string {
	return filepath.Join(c.dataDir, DefaultCloneSubdir)
}

// LiteLLMCacheDir returns the LiteLLM cache directory path.
func (c AppConfig) LiteLLMCacheDir() string {
	return filepath.Join(c.dataDir, "litellm_cache")
}

// EnsureDataDir creates the data directory if it doesn't exist.
func (c AppConfig) EnsureDataDir() error {
	return os.MkdirAll(c.dataDir, 0o755)
}

// EnsureCloneDir creates the clone directory if it doesn't exist.
func (c AppConfig) EnsureCloneDir() error {
	return os.MkdirAll(c.CloneDir(), 0o755)
}

// EnsureLiteLLMCacheDir creates the LiteLLM cache directory if it doesn't exist.
func (c AppConfig) EnsureLiteLLMCacheDir() error {
	return os.MkdirAll(c.LiteLLMCacheDir(), 0o755)
}

// AppConfigOption is a functional option for AppConfig.
type AppConfigOption func(*AppConfig)

// WithHost sets the server host.
func WithHost(host string) AppConfigOption {
	return func(c *AppConfig) { c.host = host }
}

// WithPort sets the server port.
func WithPort(port int) AppConfigOption {
	return func(c *AppConfig) { c.port = port }
}

// WithDataDir sets the data directory.
func WithDataDir(dir string) AppConfigOption {
	return func(c *AppConfig) {
		c.dataDir = dir
		// Update default DB URL when data dir changes
		if c.dbURL == "" || strings.Contains(c.dbURL, "kodit.db") {
			c.dbURL = "sqlite:///" + filepath.Join(dir, "kodit.db")
		}
	}
}

// WithDBURL sets the database URL.
func WithDBURL(url string) AppConfigOption {
	return func(c *AppConfig) { c.dbURL = url }
}

// WithLogLevel sets the log level.
func WithLogLevel(level string) AppConfigOption {
	return func(c *AppConfig) { c.logLevel = level }
}

// WithLogFormat sets the log format.
func WithLogFormat(format LogFormat) AppConfigOption {
	return func(c *AppConfig) { c.logFormat = format }
}

// WithDisableTelemetry sets telemetry state.
func WithDisableTelemetry(disabled bool) AppConfigOption {
	return func(c *AppConfig) { c.disableTelemetry = disabled }
}

// WithSkipProviderValidation sets whether to skip provider validation.
// WARNING: For testing only. Kodit requires providers for full functionality.
func WithSkipProviderValidation(skip bool) AppConfigOption {
	return func(c *AppConfig) { c.skipProviderValidation = skip }
}

// WithEmbeddingEndpoint sets the embedding endpoint.
func WithEmbeddingEndpoint(e Endpoint) AppConfigOption {
	return func(c *AppConfig) { c.embeddingEndpoint = &e }
}

// WithEnrichmentEndpoint sets the enrichment endpoint.
func WithEnrichmentEndpoint(e Endpoint) AppConfigOption {
	return func(c *AppConfig) { c.enrichmentEndpoint = &e }
}

// WithPeriodicSyncConfig sets the periodic sync config.
func WithPeriodicSyncConfig(p PeriodicSyncConfig) AppConfigOption {
	return func(c *AppConfig) { c.periodicSync = p }
}

// WithAPIKeys sets the API keys.
func WithAPIKeys(keys []string) AppConfigOption {
	return func(c *AppConfig) {
		c.apiKeys = make([]string, len(keys))
		copy(c.apiKeys, keys)
	}
}

// WithRemoteConfig sets the remote config.
func WithRemoteConfig(r RemoteConfig) AppConfigOption {
	return func(c *AppConfig) { c.remote = r }
}

// WithReportingConfig sets the reporting config.
func WithReportingConfig(r ReportingConfig) AppConfigOption {
	return func(c *AppConfig) { c.reporting = r }
}

// WithLiteLLMCacheConfig sets the LiteLLM cache config.
func WithLiteLLMCacheConfig(l LiteLLMCacheConfig) AppConfigOption {
	return func(c *AppConfig) { c.litellmCache = l }
}

// WithWorkerCount sets the number of background workers.
func WithWorkerCount(n int) AppConfigOption {
	return func(c *AppConfig) {
		if n > 0 {
			c.workerCount = n
		}
	}
}

// WithSearchLimit sets the default search result limit.
func WithSearchLimit(n int) AppConfigOption {
	return func(c *AppConfig) {
		if n > 0 {
			c.searchLimit = n
		}
	}
}

// NewAppConfigWithOptions creates an AppConfig with functional options.
func NewAppConfigWithOptions(opts ...AppConfigOption) AppConfig {
	c := NewAppConfig()
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// Apply returns a new AppConfig with the given options applied.
// This copies all fields from the receiver and then applies the options,
// making it safe to use when adding new fields to AppConfig.
func (c AppConfig) Apply(opts ...AppConfigOption) AppConfig {
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// LogAttrs returns slog attributes for logging the configuration.
// Sensitive values like API keys are masked or shown as counts.
func (c AppConfig) LogAttrs() []slog.Attr {
	return []slog.Attr{
		slog.String("data_dir", c.dataDir),
		slog.String("clone_dir", c.CloneDir()),
		slog.String("log_level", c.logLevel),
		slog.String("db_url", c.maskedDBURL()),
		slog.String("embedding_base_url", c.endpointBaseURL(c.embeddingEndpoint)),
		slog.String("embedding_model", c.endpointModel(c.embeddingEndpoint)),
		slog.String("enrichment_base_url", c.endpointBaseURL(c.enrichmentEndpoint)),
		slog.String("enrichment_model", c.endpointModel(c.enrichmentEndpoint)),
		slog.Int("api_keys_count", len(c.apiKeys)),
		slog.Bool("skip_provider_validation", c.skipProviderValidation),
		slog.Bool("periodic_sync_enabled", c.periodicSync.Enabled()),
		slog.Duration("periodic_sync_interval", c.periodicSync.Interval()),
	}
}

func (c AppConfig) maskedDBURL() string {
	if c.dbURL == "" {
		return "(default)"
	}
	if len(c.dbURL) >= 7 && c.dbURL[:7] == "sqlite:" {
		return c.dbURL
	}
	return "postgres://***@***"
}

func (c AppConfig) endpointBaseURL(e *Endpoint) string {
	if e == nil {
		return "(not configured)"
	}
	return e.BaseURL()
}

func (c AppConfig) endpointModel(e *Endpoint) string {
	if e == nil {
		return "(not configured)"
	}
	return e.Model()
}

// ParseAPIKeys parses a comma-separated string of API keys.
func ParseAPIKeys(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	return keys
}
