package kodit

import (
	"io"
	"log/slog"
	"time"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/config"
)

// databaseType identifies the database.
type databaseType int

const (
	databaseUnset databaseType = iota
	databaseSQLite
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
	modelDir               string
	textProvider           provider.TextGenerator
	embeddingProvider      provider.Embedder
	logger                 *slog.Logger
	apiKeys                []string
	workerCount            int
	workerPollPeriod       time.Duration
	skipProviderValidation bool
	embeddingBudget        search.TokenBudget
	enrichmentBudget       search.TokenBudget
	embeddingParallelism   int
	enrichmentParallelism  int
	enricherParallelism    int
	periodicSync           config.PeriodicSyncConfig
	closers                []io.Closer
}

// newClientConfig creates a clientConfig with defaults from internal/config.
// This ensures all defaults come from the single source of truth.
func newClientConfig() *clientConfig {
	return &clientConfig{
		dataDir:               config.DefaultDataDir(),
		workerCount:           config.DefaultWorkerCount,
		embeddingBudget:       search.DefaultTokenBudget(),
		enrichmentBudget:      search.DefaultTokenBudget(),
		embeddingParallelism:  1,
		enrichmentParallelism: 1,
		enricherParallelism:   1,
		periodicSync:          config.NewPeriodicSyncConfig(),
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

// WithEmbeddingBudget sets the token budget for code embedding batches.
func WithEmbeddingBudget(b search.TokenBudget) Option {
	return func(c *clientConfig) {
		c.embeddingBudget = b
	}
}

// WithEnrichmentBudget sets the token budget for enrichment embedding batches.
func WithEnrichmentBudget(b search.TokenBudget) Option {
	return func(c *clientConfig) {
		c.enrichmentBudget = b
	}
}

// WithEmbeddingParallelism sets how many embedding batches are dispatched concurrently.
// Defaults to 1. Values <= 0 are ignored.
func WithEmbeddingParallelism(n int) Option {
	return func(c *clientConfig) {
		if n > 0 {
			c.embeddingParallelism = n
		}
	}
}

// WithEnrichmentParallelism sets how many enrichment embedding batches are dispatched concurrently.
// Defaults to 1. Values <= 0 are ignored.
func WithEnrichmentParallelism(n int) Option {
	return func(c *clientConfig) {
		if n > 0 {
			c.enrichmentParallelism = n
		}
	}
}

// WithEnricherParallelism sets how many enrichment LLM requests are dispatched concurrently.
// Defaults to 1. Values <= 0 are ignored.
func WithEnricherParallelism(n int) Option {
	return func(c *clientConfig) {
		if n > 0 {
			c.enricherParallelism = n
		}
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

// WithWorkerPollPeriod sets how often the background worker checks for new tasks.
// Defaults to 1 second. Lower values speed up task processing at the cost of
// more frequent polling — useful in tests.
func WithWorkerPollPeriod(d time.Duration) Option {
	return func(c *clientConfig) {
		c.workerPollPeriod = d
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

// WithPeriodicSyncConfig sets the periodic sync configuration.
func WithPeriodicSyncConfig(cfg config.PeriodicSyncConfig) Option {
	return func(c *clientConfig) {
		c.periodicSync = cfg
	}
}

// WithModelDir sets the directory where built-in model files are stored.
// Defaults to {dataDir}/models if not specified.
func WithModelDir(dir string) Option {
	return func(c *clientConfig) {
		c.modelDir = dir
	}
}

// WithCloser registers a resource to be closed when the Client shuts down.
func WithCloser(c io.Closer) Option {
	return func(cfg *clientConfig) {
		cfg.closers = append(cfg.closers, c)
	}
}
