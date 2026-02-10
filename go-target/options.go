package kodit

import (
	"log/slog"

	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/config"
)

// databaseType identifies the database.
type databaseType int

const (
	databaseUnset databaseType = iota
	databaseSQLite
	databasePostgres
	databasePostgresPgvector
	databasePostgresVectorchord
)

// clientConfig holds configuration for Client construction.
// Use newClientConfig() to create with defaults from internal/config.
type clientConfig struct {
	database               databaseType
	dbPath                 string
	dbDSN                  string
	dataDir                string
	cloneDir               string
	textProvider           provider.TextGenerator
	embeddingProvider      provider.Embedder
	logger                 *slog.Logger
	apiKeys                []string
	workerCount            int
	skipProviderValidation bool
}

// newClientConfig creates a clientConfig with defaults from internal/config.
// This ensures all defaults come from the single source of truth.
func newClientConfig() *clientConfig {
	return &clientConfig{
		dataDir:     config.DefaultDataDir(),
		workerCount: config.DefaultWorkerCount,
	}
}

// Option configures the Client.
type Option func(*clientConfig)

// WithSQLite configures SQLite as the database.
// BM25 uses FTS5, vector search uses the configured embedding provider.
func WithSQLite(path string) Option {
	return func(c *clientConfig) {
		c.database = databaseSQLite
		c.dbPath = path
	}
}

// WithPostgres configures PostgreSQL as the database.
// Uses native PostgreSQL full-text search for BM25.
func WithPostgres(dsn string) Option {
	return func(c *clientConfig) {
		c.database = databasePostgres
		c.dbDSN = dsn
	}
}

// WithPostgresPgvector configures PostgreSQL with pgvector extension.
func WithPostgresPgvector(dsn string) Option {
	return func(c *clientConfig) {
		c.database = databasePostgresPgvector
		c.dbDSN = dsn
	}
}

// WithPostgresVectorchord configures PostgreSQL with VectorChord extension.
// VectorChord provides both BM25 and vector search.
func WithPostgresVectorchord(dsn string) Option {
	return func(c *clientConfig) {
		c.database = databasePostgresVectorchord
		c.dbDSN = dsn
	}
}

// WithOpenAI sets OpenAI as the AI provider (text + embeddings).
func WithOpenAI(apiKey string) Option {
	return func(c *clientConfig) {
		p := provider.NewOpenAIProvider(apiKey)
		c.textProvider = p
		c.embeddingProvider = p
	}
}

// WithOpenAIConfig sets OpenAI with custom configuration.
func WithOpenAIConfig(cfg provider.OpenAIConfig) Option {
	return func(c *clientConfig) {
		p := provider.NewOpenAIProviderFromConfig(cfg)
		c.textProvider = p
		c.embeddingProvider = p
	}
}

// WithAnthropic sets Anthropic Claude as the text generation provider.
// Requires a separate embedding provider since Anthropic doesn't provide embeddings.
func WithAnthropic(apiKey string) Option {
	return func(c *clientConfig) {
		p := provider.NewAnthropicProvider(apiKey)
		c.textProvider = p
	}
}

// WithAnthropicConfig sets Anthropic Claude with custom configuration.
func WithAnthropicConfig(cfg provider.AnthropicConfig) Option {
	return func(c *clientConfig) {
		p := provider.NewAnthropicProviderFromConfig(cfg)
		c.textProvider = p
	}
}

// WithTextProvider sets a custom text generation provider.
func WithTextProvider(p provider.TextGenerator) Option {
	return func(c *clientConfig) {
		c.textProvider = p
	}
}

// WithEmbeddingProvider sets a custom embedding provider.
func WithEmbeddingProvider(p provider.Embedder) Option {
	return func(c *clientConfig) {
		c.embeddingProvider = p
	}
}

// WithDataDir sets the data directory for cloned repositories and database storage.
func WithDataDir(dir string) Option {
	return func(c *clientConfig) {
		c.dataDir = dir
	}
}

// WithCloneDir sets the directory where repositories are cloned.
// If not specified, defaults to {dataDir}/repos.
func WithCloneDir(dir string) Option {
	return func(c *clientConfig) {
		c.cloneDir = dir
	}
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *clientConfig) {
		c.logger = l
	}
}

// WithAPIKeys sets the API keys for HTTP API authentication.
func WithAPIKeys(keys ...string) Option {
	return func(c *clientConfig) {
		c.apiKeys = keys
	}
}

// WithWorkerCount sets the number of background worker goroutines.
// Defaults to 1 if not specified.
func WithWorkerCount(n int) Option {
	return func(c *clientConfig) {
		if n > 0 {
			c.workerCount = n
		}
	}
}

// WithSkipProviderValidation skips the provider configuration validation.
// This is intended for testing only. In production, embedding and text
// providers are required for full functionality.
func WithSkipProviderValidation() Option {
	return func(c *clientConfig) {
		c.skipProviderValidation = true
	}
}


