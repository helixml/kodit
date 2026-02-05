package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromEnv_Defaults(t *testing.T) {
	// Clear any existing env vars that might interfere
	clearEnvVars(t)

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	// Check defaults
	assert.Equal(t, "0.0.0.0", cfg.Host)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "", cfg.DataDir)
	assert.Equal(t, "", cfg.DBURL)
	assert.Equal(t, "INFO", cfg.LogLevel)
	assert.Equal(t, "pretty", cfg.LogFormat)
	assert.False(t, cfg.DisableTelemetry)
	assert.Equal(t, "", cfg.APIKeys)
	assert.Equal(t, 1, cfg.WorkerCount)
	assert.Equal(t, 10, cfg.SearchLimit)

	// Nested struct defaults
	assert.Equal(t, "sqlite", cfg.Search.Provider)
	assert.Equal(t, "dulwich", cfg.Git.Provider)
	assert.True(t, cfg.PeriodicSync.Enabled)
	assert.Equal(t, 1800.0, cfg.PeriodicSync.IntervalSeconds)
	assert.Equal(t, 3, cfg.PeriodicSync.RetryAttempts)
	assert.Equal(t, 30.0, cfg.Remote.Timeout)
	assert.Equal(t, 3, cfg.Remote.MaxRetries)
	assert.True(t, cfg.Remote.VerifySSL)
	assert.Equal(t, 5.0, cfg.Reporting.LogTimeInterval)
	assert.True(t, cfg.LiteLLMCache.Enabled)
}

func TestEnvDefaults_MatchConfigDefaults(t *testing.T) {
	// This test verifies that struct tag defaults in env.go match the constants in config.go.
	// Go's struct tag defaults must be literals, so this test ensures they stay in sync.
	clearEnvVars(t)

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	// Core config defaults
	assert.Equal(t, DefaultHost, cfg.Host, "Host struct tag default should match DefaultHost")
	assert.Equal(t, DefaultPort, cfg.Port, "Port struct tag default should match DefaultPort")
	assert.Equal(t, DefaultLogLevel, cfg.LogLevel, "LogLevel struct tag default should match DefaultLogLevel")
	assert.Equal(t, DefaultWorkerCount, cfg.WorkerCount, "WorkerCount struct tag default should match DefaultWorkerCount")
	assert.Equal(t, DefaultSearchLimit, cfg.SearchLimit, "SearchLimit struct tag default should match DefaultSearchLimit")

	// Endpoint defaults
	assert.Equal(t, DefaultEndpointParallelTasks, cfg.EmbeddingEndpoint.NumParallelTasks, "NumParallelTasks struct tag default should match DefaultEndpointParallelTasks")
	assert.Equal(t, DefaultEndpointTimeout.Seconds(), cfg.EmbeddingEndpoint.Timeout, "Timeout struct tag default should match DefaultEndpointTimeout")
	assert.Equal(t, DefaultEndpointMaxRetries, cfg.EmbeddingEndpoint.MaxRetries, "MaxRetries struct tag default should match DefaultEndpointMaxRetries")
	assert.Equal(t, DefaultEndpointInitialDelay.Seconds(), cfg.EmbeddingEndpoint.InitialDelay, "InitialDelay struct tag default should match DefaultEndpointInitialDelay")
	assert.Equal(t, DefaultEndpointBackoffFactor, cfg.EmbeddingEndpoint.BackoffFactor, "BackoffFactor struct tag default should match DefaultEndpointBackoffFactor")
	assert.Equal(t, DefaultEndpointMaxTokens, cfg.EmbeddingEndpoint.MaxTokens, "MaxTokens struct tag default should match DefaultEndpointMaxTokens")

	// Periodic sync defaults
	assert.Equal(t, DefaultPeriodicSyncInterval, cfg.PeriodicSync.IntervalSeconds, "IntervalSeconds struct tag default should match DefaultPeriodicSyncInterval")
	assert.Equal(t, DefaultPeriodicSyncRetries, cfg.PeriodicSync.RetryAttempts, "RetryAttempts struct tag default should match DefaultPeriodicSyncRetries")

	// Remote defaults
	assert.Equal(t, DefaultRemoteTimeout.Seconds(), cfg.Remote.Timeout, "Remote.Timeout struct tag default should match DefaultRemoteTimeout")
	assert.Equal(t, DefaultRemoteMaxRetries, cfg.Remote.MaxRetries, "Remote.MaxRetries struct tag default should match DefaultRemoteMaxRetries")

	// Reporting defaults
	assert.Equal(t, DefaultReportingInterval.Seconds(), cfg.Reporting.LogTimeInterval, "LogTimeInterval struct tag default should match DefaultReportingInterval")
}

func TestLoadFromEnv_OverrideValues(t *testing.T) {
	clearEnvVars(t)

	// Set environment variables
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("PORT", "9000")
	t.Setenv("DATA_DIR", "/custom/data")
	t.Setenv("DB_URL", "postgres://localhost/kodit")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("DISABLE_TELEMETRY", "true")
	t.Setenv("API_KEYS", "key1,key2,key3")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", cfg.Host)
	assert.Equal(t, 9000, cfg.Port)
	assert.Equal(t, "/custom/data", cfg.DataDir)
	assert.Equal(t, "postgres://localhost/kodit", cfg.DBURL)
	assert.Equal(t, "DEBUG", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.True(t, cfg.DisableTelemetry)
	assert.Equal(t, "key1,key2,key3", cfg.APIKeys)
}

func TestLoadFromEnv_EmbeddingEndpoint(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("EMBEDDING_ENDPOINT_BASE_URL", "https://api.openai.com/v1")
	t.Setenv("EMBEDDING_ENDPOINT_MODEL", "text-embedding-3-small")
	t.Setenv("EMBEDDING_ENDPOINT_API_KEY", "sk-test-key")
	t.Setenv("EMBEDDING_ENDPOINT_NUM_PARALLEL_TASKS", "5")
	t.Setenv("EMBEDDING_ENDPOINT_TIMEOUT", "120")
	t.Setenv("EMBEDDING_ENDPOINT_MAX_RETRIES", "3")
	t.Setenv("EMBEDDING_ENDPOINT_INITIAL_DELAY", "1.5")
	t.Setenv("EMBEDDING_ENDPOINT_BACKOFF_FACTOR", "1.5")
	t.Setenv("EMBEDDING_ENDPOINT_MAX_TOKENS", "8000")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.True(t, cfg.EmbeddingEndpoint.IsConfigured())
	assert.Equal(t, "https://api.openai.com/v1", cfg.EmbeddingEndpoint.BaseURL)
	assert.Equal(t, "text-embedding-3-small", cfg.EmbeddingEndpoint.Model)
	assert.Equal(t, "sk-test-key", cfg.EmbeddingEndpoint.APIKey)
	assert.Equal(t, 5, cfg.EmbeddingEndpoint.NumParallelTasks)
	assert.Equal(t, 120.0, cfg.EmbeddingEndpoint.Timeout)
	assert.Equal(t, 3, cfg.EmbeddingEndpoint.MaxRetries)
	assert.Equal(t, 1.5, cfg.EmbeddingEndpoint.InitialDelay)
	assert.Equal(t, 1.5, cfg.EmbeddingEndpoint.BackoffFactor)
	assert.Equal(t, 8000, cfg.EmbeddingEndpoint.MaxTokens)
}

func TestLoadFromEnv_EnrichmentEndpoint(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("ENRICHMENT_ENDPOINT_BASE_URL", "https://api.anthropic.com/v1")
	t.Setenv("ENRICHMENT_ENDPOINT_MODEL", "claude-3-opus")
	t.Setenv("ENRICHMENT_ENDPOINT_API_KEY", "sk-anthropic-key")
	t.Setenv("ENRICHMENT_ENDPOINT_SOCKET_PATH", "/tmp/llm.sock")
	t.Setenv("ENRICHMENT_ENDPOINT_EXTRA_PARAMS", `{"temperature": 0.7, "top_p": 0.9}`)

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.True(t, cfg.EnrichmentEndpoint.IsConfigured())
	assert.Equal(t, "https://api.anthropic.com/v1", cfg.EnrichmentEndpoint.BaseURL)
	assert.Equal(t, "claude-3-opus", cfg.EnrichmentEndpoint.Model)
	assert.Equal(t, "sk-anthropic-key", cfg.EnrichmentEndpoint.APIKey)
	assert.Equal(t, "/tmp/llm.sock", cfg.EnrichmentEndpoint.SocketPath)
	assert.Equal(t, `{"temperature": 0.7, "top_p": 0.9}`, cfg.EnrichmentEndpoint.ExtraParams)
}

func TestLoadFromEnv_PeriodicSync(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("PERIODIC_SYNC_ENABLED", "false")
	t.Setenv("PERIODIC_SYNC_INTERVAL_SECONDS", "3600")
	t.Setenv("PERIODIC_SYNC_RETRY_ATTEMPTS", "5")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.False(t, cfg.PeriodicSync.Enabled)
	assert.Equal(t, 3600.0, cfg.PeriodicSync.IntervalSeconds)
	assert.Equal(t, 5, cfg.PeriodicSync.RetryAttempts)
}

func TestLoadFromEnv_Remote(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("REMOTE_SERVER_URL", "https://kodit.example.com")
	t.Setenv("REMOTE_API_KEY", "remote-api-key")
	t.Setenv("REMOTE_TIMEOUT", "60")
	t.Setenv("REMOTE_MAX_RETRIES", "5")
	t.Setenv("REMOTE_VERIFY_SSL", "false")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.True(t, cfg.Remote.IsConfigured())
	assert.Equal(t, "https://kodit.example.com", cfg.Remote.ServerURL)
	assert.Equal(t, "remote-api-key", cfg.Remote.APIKey)
	assert.Equal(t, 60.0, cfg.Remote.Timeout)
	assert.Equal(t, 5, cfg.Remote.MaxRetries)
	assert.False(t, cfg.Remote.VerifySSL)
}

func TestLoadFromEnv_Search(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("DEFAULT_SEARCH_PROVIDER", "vectorchord")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "vectorchord", cfg.Search.Provider)
}

func TestLoadFromEnv_Git(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("GIT_PROVIDER", "pygit2")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "pygit2", cfg.Git.Provider)
}

func TestLoadFromEnv_Reporting(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("REPORTING_LOG_TIME_INTERVAL", "10")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, 10.0, cfg.Reporting.LogTimeInterval)
}

func TestLoadFromEnv_LiteLLMCache(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("LITELLM_CACHE_ENABLED", "false")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.False(t, cfg.LiteLLMCache.Enabled)
}

func TestLoadFromEnv_WorkerCountAndSearchLimit(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("WORKER_COUNT", "4")
	t.Setenv("SEARCH_LIMIT", "25")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, 4, cfg.WorkerCount)
	assert.Equal(t, 25, cfg.SearchLimit)
}

func TestEnvConfig_ToAppConfig(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("DATA_DIR", "/test/data")
	t.Setenv("DB_URL", "postgres://test/db")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("DISABLE_TELEMETRY", "true")
	t.Setenv("API_KEYS", "key1,key2")
	t.Setenv("EMBEDDING_ENDPOINT_MODEL", "text-embedding-3-small")
	t.Setenv("ENRICHMENT_ENDPOINT_MODEL", "gpt-4")
	t.Setenv("DEFAULT_SEARCH_PROVIDER", "vectorchord")
	t.Setenv("PERIODIC_SYNC_ENABLED", "false")
	t.Setenv("REMOTE_SERVER_URL", "https://remote.example.com")

	envCfg, err := LoadFromEnv()
	require.NoError(t, err)

	cfg := envCfg.ToAppConfig()

	assert.Equal(t, "/test/data", cfg.DataDir())
	assert.Equal(t, "postgres://test/db", cfg.DBURL())
	assert.Equal(t, "DEBUG", cfg.LogLevel())
	assert.Equal(t, LogFormatJSON, cfg.LogFormat())
	assert.True(t, cfg.DisableTelemetry())
	assert.Equal(t, []string{"key1", "key2"}, cfg.APIKeys())
	assert.NotNil(t, cfg.EmbeddingEndpoint())
	assert.Equal(t, "text-embedding-3-small", cfg.EmbeddingEndpoint().Model())
	assert.NotNil(t, cfg.EnrichmentEndpoint())
	assert.Equal(t, "gpt-4", cfg.EnrichmentEndpoint().Model())
	assert.Equal(t, SearchProviderVectorChord, cfg.Search().Provider())
	assert.False(t, cfg.PeriodicSync().Enabled())
	assert.True(t, cfg.Remote().IsConfigured())
	assert.Equal(t, "https://remote.example.com", cfg.Remote().ServerURL())
}

func TestEndpointEnv_ToEndpoint(t *testing.T) {
	env := EndpointEnv{
		BaseURL:          "https://api.example.com",
		Model:            "test-model",
		APIKey:           "test-key",
		NumParallelTasks: 5,
		SocketPath:       "/tmp/socket",
		Timeout:          120,
		MaxRetries:       3,
		InitialDelay:     1.5,
		BackoffFactor:    1.5,
		ExtraParams:      `{"key": "value"}`,
		MaxTokens:        8000,
	}

	endpoint := env.ToEndpoint()

	assert.Equal(t, "https://api.example.com", endpoint.BaseURL())
	assert.Equal(t, "test-model", endpoint.Model())
	assert.Equal(t, "test-key", endpoint.APIKey())
	assert.Equal(t, 5, endpoint.NumParallelTasks())
	assert.Equal(t, "/tmp/socket", endpoint.SocketPath())
	assert.Equal(t, 120*time.Second, endpoint.Timeout())
	assert.Equal(t, 3, endpoint.MaxRetries())
	assert.Equal(t, time.Duration(1.5*float64(time.Second)), endpoint.InitialDelay())
	assert.Equal(t, 1.5, endpoint.BackoffFactor())
	assert.Equal(t, map[string]any{"key": "value"}, endpoint.ExtraParams())
	assert.Equal(t, 8000, endpoint.MaxTokens())
}

func TestParseLogFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected LogFormat
	}{
		{"json", LogFormatJSON},
		{"JSON", LogFormatJSON},
		{"pretty", LogFormatPretty},
		{"PRETTY", LogFormatPretty},
		{"", LogFormatPretty},
		{"invalid", LogFormatPretty},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, parseLogFormat(tc.input))
		})
	}
}

func TestParseExtraParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]any
	}{
		{
			name:     "valid json",
			input:    `{"temperature": 0.7, "top_p": 0.9}`,
			expected: map[string]any{"temperature": 0.7, "top_p": 0.9},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "invalid json",
			input:    "not json",
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseExtraParams(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestLoadDotEnv(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := `DATA_DIR=/from/dotenv
LOG_LEVEL=DEBUG
API_KEYS=key1,key2
`
	err := os.WriteFile(envFile, []byte(content), 0o644)
	require.NoError(t, err)

	clearEnvVars(t)

	// Load .env file
	err = LoadDotEnv(envFile)
	require.NoError(t, err)

	// Verify env vars were loaded
	assert.Equal(t, "/from/dotenv", os.Getenv("DATA_DIR"))
	assert.Equal(t, "DEBUG", os.Getenv("LOG_LEVEL"))
	assert.Equal(t, "key1,key2", os.Getenv("API_KEYS"))
}

func TestLoadDotEnv_NonExistent(t *testing.T) {
	clearEnvVars(t)

	// Should not error for non-existent file
	err := LoadDotEnv("/nonexistent/.env")
	assert.NoError(t, err)
}

func TestMustLoadDotEnv_NonExistent(t *testing.T) {
	clearEnvVars(t)

	// Should error for non-existent file
	err := MustLoadDotEnv("/nonexistent/.env")
	assert.Error(t, err)
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := `DATA_DIR=/config/data
LOG_LEVEL=WARN
EMBEDDING_ENDPOINT_MODEL=test-embedding
`
	err := os.WriteFile(envFile, []byte(content), 0o644)
	require.NoError(t, err)

	clearEnvVars(t)

	// Load full config
	cfg, err := LoadConfig(envFile)
	require.NoError(t, err)

	assert.Equal(t, "/config/data", cfg.DataDir())
	assert.Equal(t, "WARN", cfg.LogLevel())
	assert.NotNil(t, cfg.EmbeddingEndpoint())
	assert.Equal(t, "test-embedding", cfg.EmbeddingEndpoint().Model())
}

func TestLoadDotEnvFromFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first .env file
	env1 := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(env1, []byte("KEY1=value1\nKEY2=value2\n"), 0o644)
	require.NoError(t, err)

	// Create second .env file
	env2 := filepath.Join(tmpDir, ".env.local")
	err = os.WriteFile(env2, []byte("KEY2=override\nKEY3=value3\n"), 0o644)
	require.NoError(t, err)

	clearEnvVars(t)

	// Load multiple files - note: godotenv.Load does NOT override existing values
	// so KEY2 keeps its value from env1
	err = LoadDotEnvFromFiles(env1, env2)
	require.NoError(t, err)

	assert.Equal(t, "value1", os.Getenv("KEY1"))
	assert.Equal(t, "value2", os.Getenv("KEY2")) // First file wins
	assert.Equal(t, "value3", os.Getenv("KEY3"))
}

func TestOverloadDotEnvFromFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first .env file
	env1 := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(env1, []byte("KEY1=value1\nKEY2=value2\n"), 0o644)
	require.NoError(t, err)

	// Create second .env file (will override KEY2)
	env2 := filepath.Join(tmpDir, ".env.local")
	err = os.WriteFile(env2, []byte("KEY2=override\nKEY3=value3\n"), 0o644)
	require.NoError(t, err)

	clearEnvVars(t)

	// Overload multiple files - later files override earlier values
	err = OverloadDotEnvFromFiles(env1, env2)
	require.NoError(t, err)

	assert.Equal(t, "value1", os.Getenv("KEY1"))
	assert.Equal(t, "override", os.Getenv("KEY2")) // Second file wins with Overload
	assert.Equal(t, "value3", os.Getenv("KEY3"))
}

// clearEnvVars unsets all config-related environment variables
func clearEnvVars(t *testing.T) {
	t.Helper()

	vars := []string{
		"HOST",
		"PORT",
		"DATA_DIR",
		"DB_URL",
		"LOG_LEVEL",
		"LOG_FORMAT",
		"DISABLE_TELEMETRY",
		"API_KEYS",
		"EMBEDDING_ENDPOINT_BASE_URL",
		"EMBEDDING_ENDPOINT_MODEL",
		"EMBEDDING_ENDPOINT_API_KEY",
		"EMBEDDING_ENDPOINT_NUM_PARALLEL_TASKS",
		"EMBEDDING_ENDPOINT_SOCKET_PATH",
		"EMBEDDING_ENDPOINT_TIMEOUT",
		"EMBEDDING_ENDPOINT_MAX_RETRIES",
		"EMBEDDING_ENDPOINT_INITIAL_DELAY",
		"EMBEDDING_ENDPOINT_BACKOFF_FACTOR",
		"EMBEDDING_ENDPOINT_EXTRA_PARAMS",
		"EMBEDDING_ENDPOINT_MAX_TOKENS",
		"ENRICHMENT_ENDPOINT_BASE_URL",
		"ENRICHMENT_ENDPOINT_MODEL",
		"ENRICHMENT_ENDPOINT_API_KEY",
		"ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS",
		"ENRICHMENT_ENDPOINT_SOCKET_PATH",
		"ENRICHMENT_ENDPOINT_TIMEOUT",
		"ENRICHMENT_ENDPOINT_MAX_RETRIES",
		"ENRICHMENT_ENDPOINT_INITIAL_DELAY",
		"ENRICHMENT_ENDPOINT_BACKOFF_FACTOR",
		"ENRICHMENT_ENDPOINT_EXTRA_PARAMS",
		"ENRICHMENT_ENDPOINT_MAX_TOKENS",
		"DEFAULT_SEARCH_PROVIDER",
		"GIT_PROVIDER",
		"PERIODIC_SYNC_ENABLED",
		"PERIODIC_SYNC_INTERVAL_SECONDS",
		"PERIODIC_SYNC_RETRY_ATTEMPTS",
		"REMOTE_SERVER_URL",
		"REMOTE_API_KEY",
		"REMOTE_TIMEOUT",
		"REMOTE_MAX_RETRIES",
		"REMOTE_VERIFY_SSL",
		"REPORTING_LOG_TIME_INTERVAL",
		"LITELLM_CACHE_ENABLED",
		"WORKER_COUNT",
		"SEARCH_LIMIT",
		"KEY1",
		"KEY2",
		"KEY3",
	}

	for _, v := range vars {
		_ = os.Unsetenv(v)
	}
}
