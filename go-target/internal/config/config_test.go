package config

import (
	"testing"
	"time"
)

func TestDefaultConstants(t *testing.T) {
	if DefaultWorkerCount != 1 {
		t.Errorf("DefaultWorkerCount = %v, want 1", DefaultWorkerCount)
	}
	if DefaultSearchLimit != 10 {
		t.Errorf("DefaultSearchLimit = %v, want 10", DefaultSearchLimit)
	}
	if DefaultHost != "0.0.0.0" {
		t.Errorf("DefaultHost = %v, want '0.0.0.0'", DefaultHost)
	}
	if DefaultPort != 8080 {
		t.Errorf("DefaultPort = %v, want 8080", DefaultPort)
	}
	if DefaultLogLevel != "INFO" {
		t.Errorf("DefaultLogLevel = %v, want 'INFO'", DefaultLogLevel)
	}
	if DefaultCloneSubdir != "repos" {
		t.Errorf("DefaultCloneSubdir = %v, want 'repos'", DefaultCloneSubdir)
	}
	if DefaultEndpointParallelTasks != 10 {
		t.Errorf("DefaultEndpointParallelTasks = %v, want 10", DefaultEndpointParallelTasks)
	}
	if DefaultEndpointTimeout != 60*time.Second {
		t.Errorf("DefaultEndpointTimeout = %v, want 60s", DefaultEndpointTimeout)
	}
	if DefaultEndpointMaxRetries != 5 {
		t.Errorf("DefaultEndpointMaxRetries = %v, want 5", DefaultEndpointMaxRetries)
	}
	if DefaultEndpointInitialDelay != 2*time.Second {
		t.Errorf("DefaultEndpointInitialDelay = %v, want 2s", DefaultEndpointInitialDelay)
	}
	if DefaultEndpointBackoffFactor != 2.0 {
		t.Errorf("DefaultEndpointBackoffFactor = %v, want 2.0", DefaultEndpointBackoffFactor)
	}
	if DefaultEndpointMaxTokens != 4000 {
		t.Errorf("DefaultEndpointMaxTokens = %v, want 4000", DefaultEndpointMaxTokens)
	}
	if DefaultPeriodicSyncInterval != 1800.0 {
		t.Errorf("DefaultPeriodicSyncInterval = %v, want 1800.0", DefaultPeriodicSyncInterval)
	}
	if DefaultPeriodicSyncRetries != 3 {
		t.Errorf("DefaultPeriodicSyncRetries = %v, want 3", DefaultPeriodicSyncRetries)
	}
	if DefaultRemoteTimeout != 30*time.Second {
		t.Errorf("DefaultRemoteTimeout = %v, want 30s", DefaultRemoteTimeout)
	}
	if DefaultRemoteMaxRetries != 3 {
		t.Errorf("DefaultRemoteMaxRetries = %v, want 3", DefaultRemoteMaxRetries)
	}
	if DefaultReportingInterval != 5*time.Second {
		t.Errorf("DefaultReportingInterval = %v, want 5s", DefaultReportingInterval)
	}
}

func TestReportingConfig(t *testing.T) {
	cfg := NewReportingConfig()

	if cfg.LogTimeInterval() != DefaultReportingInterval {
		t.Errorf("LogTimeInterval() = %v, want %v", cfg.LogTimeInterval(), DefaultReportingInterval)
	}

	cfg = cfg.WithLogTimeInterval(10 * time.Second)
	if cfg.LogTimeInterval() != 10*time.Second {
		t.Errorf("LogTimeInterval() = %v, want 10s", cfg.LogTimeInterval())
	}
}

func TestLiteLLMCacheConfig(t *testing.T) {
	cfg := NewLiteLLMCacheConfig()

	if !cfg.Enabled() {
		t.Error("Enabled() should be true by default")
	}

	cfg = cfg.WithEnabled(false)
	if cfg.Enabled() {
		t.Error("Enabled() should be false after WithEnabled(false)")
	}
}

func TestEndpoint_Defaults(t *testing.T) {
	e := NewEndpoint()

	if e.NumParallelTasks() != DefaultEndpointParallelTasks {
		t.Errorf("NumParallelTasks() = %v, want %v", e.NumParallelTasks(), DefaultEndpointParallelTasks)
	}
	if e.Timeout() != DefaultEndpointTimeout {
		t.Errorf("Timeout() = %v, want %v", e.Timeout(), DefaultEndpointTimeout)
	}
	if e.MaxRetries() != DefaultEndpointMaxRetries {
		t.Errorf("MaxRetries() = %v, want %v", e.MaxRetries(), DefaultEndpointMaxRetries)
	}
	if e.InitialDelay() != DefaultEndpointInitialDelay {
		t.Errorf("InitialDelay() = %v, want %v", e.InitialDelay(), DefaultEndpointInitialDelay)
	}
	if e.BackoffFactor() != DefaultEndpointBackoffFactor {
		t.Errorf("BackoffFactor() = %v, want %v", e.BackoffFactor(), DefaultEndpointBackoffFactor)
	}
	if e.MaxTokens() != DefaultEndpointMaxTokens {
		t.Errorf("MaxTokens() = %v, want %v", e.MaxTokens(), DefaultEndpointMaxTokens)
	}
	if e.IsConfigured() {
		t.Error("IsConfigured() should be false for default endpoint")
	}
}

func TestEndpoint_WithOptions(t *testing.T) {
	e := NewEndpointWithOptions(
		WithBaseURL("https://api.example.com"),
		WithModel("gpt-4"),
		WithAPIKey("test-key"),
		WithNumParallelTasks(20),
		WithTimeout(30*time.Second),
		WithMaxRetries(3),
	)

	if e.BaseURL() != "https://api.example.com" {
		t.Errorf("BaseURL() = %v, want 'https://api.example.com'", e.BaseURL())
	}
	if e.Model() != "gpt-4" {
		t.Errorf("Model() = %v, want 'gpt-4'", e.Model())
	}
	if e.APIKey() != "test-key" {
		t.Errorf("APIKey() = %v, want 'test-key'", e.APIKey())
	}
	if e.NumParallelTasks() != 20 {
		t.Errorf("NumParallelTasks() = %v, want 20", e.NumParallelTasks())
	}
	if e.Timeout() != 30*time.Second {
		t.Errorf("Timeout() = %v, want 30s", e.Timeout())
	}
	if e.MaxRetries() != 3 {
		t.Errorf("MaxRetries() = %v, want 3", e.MaxRetries())
	}
	if !e.IsConfigured() {
		t.Error("IsConfigured() should be true when model is set")
	}
}

func TestEndpoint_ExtraParams(t *testing.T) {
	params := map[string]any{"key": "value"}
	e := NewEndpointWithOptions(WithExtraParams(params))

	result := e.ExtraParams()
	if result["key"] != "value" {
		t.Errorf("ExtraParams()[key] = %v, want 'value'", result["key"])
	}

	// Verify returned map is a copy
	result["key"] = "modified"
	if e.ExtraParams()["key"] == "modified" {
		t.Error("ExtraParams() should return a copy")
	}
}

func TestEndpoint_ExtraParams_Nil(t *testing.T) {
	e := NewEndpoint()
	if e.ExtraParams() != nil {
		t.Error("ExtraParams() should be nil when not set")
	}
}

func TestSearchConfig(t *testing.T) {
	cfg := NewSearchConfig()

	if cfg.Provider() != SearchProviderSQLite {
		t.Errorf("Provider() = %v, want sqlite", cfg.Provider())
	}

	cfg = cfg.WithProvider(SearchProviderVectorChord)
	if cfg.Provider() != SearchProviderVectorChord {
		t.Errorf("Provider() = %v, want vectorchord", cfg.Provider())
	}
}

func TestGitConfig(t *testing.T) {
	cfg := NewGitConfig()

	if cfg.Provider() != GitProviderDulwich {
		t.Errorf("Provider() = %v, want dulwich", cfg.Provider())
	}

	cfg = cfg.WithProvider(GitProviderPygit2)
	if cfg.Provider() != GitProviderPygit2 {
		t.Errorf("Provider() = %v, want pygit2", cfg.Provider())
	}
}

func TestPeriodicSyncConfig(t *testing.T) {
	cfg := NewPeriodicSyncConfig()

	if !cfg.Enabled() {
		t.Error("Enabled() should be true by default")
	}
	expectedInterval := time.Duration(DefaultPeriodicSyncInterval * float64(time.Second))
	if cfg.Interval() != expectedInterval {
		t.Errorf("Interval() = %v, want %v", cfg.Interval(), expectedInterval)
	}
	if cfg.RetryAttempts() != DefaultPeriodicSyncRetries {
		t.Errorf("RetryAttempts() = %v, want %v", cfg.RetryAttempts(), DefaultPeriodicSyncRetries)
	}

	cfg = cfg.WithEnabled(false).WithIntervalSeconds(3600).WithRetryAttempts(5)
	if cfg.Enabled() {
		t.Error("Enabled() should be false")
	}
	if cfg.Interval() != 1*time.Hour {
		t.Errorf("Interval() = %v, want 1h", cfg.Interval())
	}
	if cfg.RetryAttempts() != 5 {
		t.Errorf("RetryAttempts() = %v, want 5", cfg.RetryAttempts())
	}
}

func TestRemoteConfig(t *testing.T) {
	cfg := NewRemoteConfig()

	if cfg.IsConfigured() {
		t.Error("IsConfigured() should be false by default")
	}
	if cfg.Timeout() != DefaultRemoteTimeout {
		t.Errorf("Timeout() = %v, want %v", cfg.Timeout(), DefaultRemoteTimeout)
	}
	if cfg.MaxRetries() != DefaultRemoteMaxRetries {
		t.Errorf("MaxRetries() = %v, want %v", cfg.MaxRetries(), DefaultRemoteMaxRetries)
	}
	if !cfg.VerifySSL() {
		t.Error("VerifySSL() should be true by default")
	}
}

func TestRemoteConfig_WithOptions(t *testing.T) {
	cfg := NewRemoteConfigWithOptions(
		WithServerURL("https://remote.example.com"),
		WithRemoteAPIKey("remote-key"),
		WithRemoteTimeout(60*time.Second),
		WithRemoteMaxRetries(5),
		WithVerifySSL(false),
	)

	if cfg.ServerURL() != "https://remote.example.com" {
		t.Errorf("ServerURL() = %v, want 'https://remote.example.com'", cfg.ServerURL())
	}
	if cfg.APIKey() != "remote-key" {
		t.Errorf("APIKey() = %v, want 'remote-key'", cfg.APIKey())
	}
	if cfg.Timeout() != 60*time.Second {
		t.Errorf("Timeout() = %v, want 60s", cfg.Timeout())
	}
	if cfg.MaxRetries() != 5 {
		t.Errorf("MaxRetries() = %v, want 5", cfg.MaxRetries())
	}
	if cfg.VerifySSL() {
		t.Error("VerifySSL() should be false")
	}
	if !cfg.IsConfigured() {
		t.Error("IsConfigured() should be true when server URL is set")
	}
}

func TestAppConfig_Defaults(t *testing.T) {
	cfg := NewAppConfig()

	if cfg.Host() != DefaultHost {
		t.Errorf("Host() = %v, want '%v'", cfg.Host(), DefaultHost)
	}
	if cfg.Port() != DefaultPort {
		t.Errorf("Port() = %v, want %v", cfg.Port(), DefaultPort)
	}
	if cfg.LogLevel() != DefaultLogLevel {
		t.Errorf("LogLevel() = %v, want '%v'", cfg.LogLevel(), DefaultLogLevel)
	}
	if cfg.LogFormat() != LogFormatPretty {
		t.Errorf("LogFormat() = %v, want 'pretty'", cfg.LogFormat())
	}
	if cfg.DisableTelemetry() {
		t.Error("DisableTelemetry() should be false by default")
	}
	if cfg.EmbeddingEndpoint() != nil {
		t.Error("EmbeddingEndpoint() should be nil by default")
	}
	if cfg.EnrichmentEndpoint() != nil {
		t.Error("EnrichmentEndpoint() should be nil by default")
	}
	if cfg.IsRemote() {
		t.Error("IsRemote() should be false by default")
	}
	if cfg.WorkerCount() != DefaultWorkerCount {
		t.Errorf("WorkerCount() = %v, want %v", cfg.WorkerCount(), DefaultWorkerCount)
	}
	if cfg.SearchLimit() != DefaultSearchLimit {
		t.Errorf("SearchLimit() = %v, want %v", cfg.SearchLimit(), DefaultSearchLimit)
	}
}

func TestAppConfig_WithOptions(t *testing.T) {
	embeddingEndpoint := NewEndpointWithOptions(WithModel("embed-model"))
	enrichmentEndpoint := NewEndpointWithOptions(WithModel("enrich-model"))
	remoteConfig := NewRemoteConfigWithOptions(WithServerURL("https://remote.com"))

	cfg := NewAppConfigWithOptions(
		WithDataDir("/custom/data"),
		WithDBURL("postgres://localhost/kodit"),
		WithLogLevel("DEBUG"),
		WithLogFormat(LogFormatJSON),
		WithDisableTelemetry(true),
		WithEmbeddingEndpoint(embeddingEndpoint),
		WithEnrichmentEndpoint(enrichmentEndpoint),
		WithAPIKeys([]string{"key1", "key2"}),
		WithRemoteConfig(remoteConfig),
	)

	if cfg.DataDir() != "/custom/data" {
		t.Errorf("DataDir() = %v, want '/custom/data'", cfg.DataDir())
	}
	if cfg.DBURL() != "postgres://localhost/kodit" {
		t.Errorf("DBURL() = %v, want 'postgres://localhost/kodit'", cfg.DBURL())
	}
	if cfg.LogLevel() != "DEBUG" {
		t.Errorf("LogLevel() = %v, want 'DEBUG'", cfg.LogLevel())
	}
	if cfg.LogFormat() != LogFormatJSON {
		t.Errorf("LogFormat() = %v, want 'json'", cfg.LogFormat())
	}
	if !cfg.DisableTelemetry() {
		t.Error("DisableTelemetry() should be true")
	}
	if cfg.EmbeddingEndpoint() == nil {
		t.Error("EmbeddingEndpoint() should not be nil")
	}
	if cfg.EnrichmentEndpoint() == nil {
		t.Error("EnrichmentEndpoint() should not be nil")
	}
	if len(cfg.APIKeys()) != 2 {
		t.Errorf("APIKeys() length = %v, want 2", len(cfg.APIKeys()))
	}
	if cfg.IsRemote() != true {
		t.Error("IsRemote() should be true when server URL is configured")
	}
}

func TestAppConfig_APIKeys_Copy(t *testing.T) {
	cfg := NewAppConfigWithOptions(WithAPIKeys([]string{"key1"}))

	keys := cfg.APIKeys()
	keys[0] = "modified"

	if cfg.APIKeys()[0] == "modified" {
		t.Error("APIKeys() should return a copy")
	}
}

func TestAppConfig_Directories(t *testing.T) {
	cfg := NewAppConfigWithOptions(WithDataDir("/data"))

	if cfg.CloneDir() != "/data/repos" {
		t.Errorf("CloneDir() = %v, want '/data/repos'", cfg.CloneDir())
	}
	if cfg.LiteLLMCacheDir() != "/data/litellm_cache" {
		t.Errorf("LiteLLMCacheDir() = %v, want '/data/litellm_cache'", cfg.LiteLLMCacheDir())
	}
}

func TestAppConfig_DataDirUpdatesDBURL(t *testing.T) {
	cfg := NewAppConfigWithOptions(WithDataDir("/custom"))

	// DB URL should be updated when only data dir is set
	expected := "sqlite:////custom/kodit.db"
	if cfg.DBURL() != expected {
		t.Errorf("DBURL() = %v, want %v", cfg.DBURL(), expected)
	}
}

func TestParseAPIKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single key",
			input:    "key1",
			expected: []string{"key1"},
		},
		{
			name:     "multiple keys",
			input:    "key1,key2,key3",
			expected: []string{"key1", "key2", "key3"},
		},
		{
			name:     "with whitespace",
			input:    "key1 , key2 , key3",
			expected: []string{"key1", "key2", "key3"},
		},
		{
			name:     "with empty entries",
			input:    "key1,,key2",
			expected: []string{"key1", "key2"},
		},
		{
			name:     "whitespace only entries",
			input:    "key1,  ,key2",
			expected: []string{"key1", "key2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAPIKeys(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("ParseAPIKeys(%q) length = %v, want %v", tt.input, len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("ParseAPIKeys(%q)[%d] = %v, want %v", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}
