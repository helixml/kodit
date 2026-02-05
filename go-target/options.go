package kodit

import (
	"log/slog"
	"path/filepath"
	"time"

	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/config"
)

// storageType identifies the storage backend.
type storageType int

const (
	storageUnset storageType = iota
	storageSQLite
	storagePostgres
	storagePostgresPgvector
	storagePostgresVectorchord
)

// clientConfig holds configuration for Client construction.
// Use newClientConfig() to create with defaults from internal/config.
type clientConfig struct {
	storage                storageType
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
		workerCount: config.DefaultWorkerCount,
	}
}

// defaultCloneDir returns the default clone directory for a given data directory.
func defaultCloneDir(dataDir string) string {
	return filepath.Join(dataDir, config.DefaultCloneSubdir)
}

// Option configures the Client.
type Option func(*clientConfig)

// WithSQLite configures SQLite as the storage backend.
// BM25 uses FTS5, vector search uses the configured embedding provider.
func WithSQLite(path string) Option {
	return func(c *clientConfig) {
		c.storage = storageSQLite
		c.dbPath = path
	}
}

// WithPostgres configures PostgreSQL as the storage backend.
// Uses native PostgreSQL full-text search for BM25.
func WithPostgres(dsn string) Option {
	return func(c *clientConfig) {
		c.storage = storagePostgres
		c.dbDSN = dsn
	}
}

// WithPostgresPgvector configures PostgreSQL with pgvector extension.
func WithPostgresPgvector(dsn string) Option {
	return func(c *clientConfig) {
		c.storage = storagePostgresPgvector
		c.dbDSN = dsn
	}
}

// WithPostgresVectorchord configures PostgreSQL with VectorChord extension.
// VectorChord provides both BM25 and vector search.
func WithPostgresVectorchord(dsn string) Option {
	return func(c *clientConfig) {
		c.storage = storagePostgresVectorchord
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

// SearchOption configures a search request.
type SearchOption func(*searchConfig)

// searchConfig holds search parameters.
// Use newSearchConfig() to create with defaults from internal/config.
type searchConfig struct {
	semanticWeight   float64
	limit            int
	offset           int
	languages        []string
	repositories     []int64
	enrichmentTypes  []string
	minScore         float64
	includeSnippets  bool
	includeDocuments bool
}

// newSearchConfig creates a searchConfig with defaults from internal/config.
func newSearchConfig() *searchConfig {
	return &searchConfig{
		limit: config.DefaultSearchLimit,
	}
}

// WithSemanticWeight sets the weight for semantic (vector) search (0-1).
// Higher values favor semantic similarity, lower values favor keyword matching.
func WithSemanticWeight(w float64) SearchOption {
	return func(c *searchConfig) {
		if w >= 0 && w <= 1 {
			c.semanticWeight = w
		}
	}
}

// WithLimit sets the maximum number of results.
func WithLimit(n int) SearchOption {
	return func(c *searchConfig) {
		if n > 0 {
			c.limit = n
		}
	}
}

// WithOffset sets the offset for pagination.
func WithOffset(n int) SearchOption {
	return func(c *searchConfig) {
		if n >= 0 {
			c.offset = n
		}
	}
}

// WithLanguages filters results by programming languages.
func WithLanguages(langs ...string) SearchOption {
	return func(c *searchConfig) {
		c.languages = langs
	}
}

// WithRepositories filters results by repository IDs.
func WithRepositories(ids ...int64) SearchOption {
	return func(c *searchConfig) {
		c.repositories = ids
	}
}

// WithEnrichmentTypes includes specific enrichment types in results.
func WithEnrichmentTypes(types ...string) SearchOption {
	return func(c *searchConfig) {
		c.enrichmentTypes = types
	}
}

// WithMinScore filters results below a minimum score threshold.
func WithMinScore(score float64) SearchOption {
	return func(c *searchConfig) {
		if score >= 0 {
			c.minScore = score
		}
	}
}

// WithSnippets includes code snippets in search results.
func WithSnippets(include bool) SearchOption {
	return func(c *searchConfig) {
		c.includeSnippets = include
	}
}

// WithDocuments includes enrichment documents in search results.
func WithDocuments(include bool) SearchOption {
	return func(c *searchConfig) {
		c.includeDocuments = include
	}
}

// EnrichmentOption configures enrichment queries.
type EnrichmentOption func(*enrichmentConfig)

// enrichmentConfig holds enrichment query parameters.
type enrichmentConfig struct {
	types    []string
	subtypes []string
	limit    int
	offset   int
}

// WithEnrichmentType filters enrichments by type.
func WithEnrichmentType(t string) EnrichmentOption {
	return func(c *enrichmentConfig) {
		c.types = append(c.types, t)
	}
}

// WithEnrichmentSubtype filters enrichments by subtype.
func WithEnrichmentSubtype(s string) EnrichmentOption {
	return func(c *enrichmentConfig) {
		c.subtypes = append(c.subtypes, s)
	}
}

// WithEnrichmentLimit sets the maximum number of enrichments to return.
func WithEnrichmentLimit(n int) EnrichmentOption {
	return func(c *enrichmentConfig) {
		if n > 0 {
			c.limit = n
		}
	}
}

// WithEnrichmentOffset sets the offset for pagination.
func WithEnrichmentOffset(n int) EnrichmentOption {
	return func(c *enrichmentConfig) {
		if n >= 0 {
			c.offset = n
		}
	}
}

// TaskOption configures task queries.
type TaskOption func(*taskConfig)

// taskConfig holds task query parameters.
type taskConfig struct {
	operations []string
	statuses   []string
	repoID     int64
	limit      int
	offset     int
	sinceTime  time.Time
}

// WithTaskOperation filters tasks by operation type.
func WithTaskOperation(op string) TaskOption {
	return func(c *taskConfig) {
		c.operations = append(c.operations, op)
	}
}

// WithTaskStatus filters tasks by status.
func WithTaskStatus(s string) TaskOption {
	return func(c *taskConfig) {
		c.statuses = append(c.statuses, s)
	}
}

// WithTaskRepository filters tasks by repository ID.
func WithTaskRepository(repoID int64) TaskOption {
	return func(c *taskConfig) {
		c.repoID = repoID
	}
}

// WithTaskLimit sets the maximum number of tasks to return.
func WithTaskLimit(n int) TaskOption {
	return func(c *taskConfig) {
		if n > 0 {
			c.limit = n
		}
	}
}

// WithTaskOffset sets the offset for pagination.
func WithTaskOffset(n int) TaskOption {
	return func(c *taskConfig) {
		if n >= 0 {
			c.offset = n
		}
	}
}

// WithTaskSince filters tasks created after a time.
func WithTaskSince(t time.Time) TaskOption {
	return func(c *taskConfig) {
		c.sinceTime = t
	}
}
