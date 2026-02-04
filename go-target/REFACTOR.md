# Kodit Library Refactor Plan

This document outlines the refactoring of Kodit from an application-centric structure to a library-first architecture that can be embedded in downstream projects while retaining full functionality.

---

## Goals

1. **Library-first design**: Kodit should be usable as an importable Go library with a clean public API
2. **Optional HTTP server**: API server becomes an optional feature, not the primary interface
3. **Richer domain model**: Treat all aggregates (Repository, Commit, Snippet, Enrichment) as first-class citizens
4. **Clear bounded contexts**: Domain, application, and infrastructure layers properly separated
5. **Functional options pattern**: Configure the library through composable options
6. **Preserve all existing functionality**: Nothing lost from the Python migration

---

## Design Principles

### Library Usage

```go
// Embedded usage (no HTTP server)
client, _ := kodit.New(
    kodit.WithSQLite("~/.kodit/data.db"),
    kodit.WithOpenAI(apiKey),
)

// Index a repository
client.Repositories().Clone(ctx, "https://github.com/kubernetes/kubernetes")

// Hybrid search
results, _ := client.Search(ctx, "create a deployment",
    search.WithSemanticWeight(0.7),
    search.WithEnrichmentTypes(enrichment.TypeSnippet, enrichment.TypeExample),
)

// Iterate results
for _, snippet := range results.Snippets() {
    fmt.Println(snippet.Path(), snippet.Name())
}

// Or start HTTP API for web clients
client.API().ListenAndServe(":8080")
```

### Key Insight

All key entities are aggregate roots at the same DDD level: Repository, Commit, Snippet, Enrichment, Task. The domain model should reflect this equality rather than privileging one over others.

---

## Target Directory Structure

```
kodit/
├── kodit.go                          # Client, main entry point
├── options.go                        # Functional options
├── errors.go                         # Exported errors
│
├── domain/
│   ├── repository/                   # Git Repository aggregate root
│   │   ├── repository.go             # Repository aggregate
│   │   ├── commit.go                 # Commit entity
│   │   ├── file.go                   # File entity
│   │   ├── branch.go                 # Branch entity
│   │   ├── tag.go                    # Tag entity
│   │   ├── author.go                 # Author value object
│   │   ├── working_copy.go           # WorkingCopy value object
│   │   ├── tracking_config.go        # TrackingConfig value object
│   │   └── store.go                  # Repository store interface (port)
│   │
│   ├── snippet/                      # Snippet aggregate root
│   │   ├── snippet.go                # Snippet aggregate (content-addressed)
│   │   ├── index.go                  # CommitIndex aggregate
│   │   ├── language.go               # Language value object
│   │   └── store.go                  # Snippet store interface (port)
│   │
│   ├── enrichment/                   # Enrichment aggregate root
│   │   ├── enrichment.go             # Base Enrichment, Type/Subtype
│   │   ├── association.go            # EnrichmentAssociation value object
│   │   ├── development.go            # DevelopmentEnrichment (snippet_summary, example)
│   │   ├── architecture.go           # ArchitectureEnrichment (physical, database_schema)
│   │   ├── history.go                # HistoryEnrichment (commit_description)
│   │   ├── usage.go                  # UsageEnrichment (cookbook, api_docs)
│   │   └── store.go                  # Enrichment store interface (port)
│   │
│   ├── task/                         # Task aggregate root
│   │   ├── task.go                   # Task entity
│   │   ├── status.go                 # TaskStatus entity
│   │   ├── operation.go              # TaskOperation value object
│   │   └── store.go                  # Task store interface (port)
│   │
│   ├── search/                       # Search domain
│   │   ├── query.go                  # Query value object
│   │   ├── result.go                 # SearchResult value object
│   │   ├── filters.go                # SearchFilters value object
│   │   ├── fusion.go                 # FusionService (RRF algorithm)
│   │   ├── bm25.go                   # BM25Repository interface (port)
│   │   └── vector.go                 # VectorRepository interface (port)
│   │
│   ├── tracking/                     # Progress tracking domain
│   │   ├── trackable.go              # Trackable interface
│   │   ├── status.go                 # RepositoryStatusSummary
│   │   └── resolution.go             # TrackableResolution
│   │
│   └── service/                      # Domain services (pure business logic)
│       ├── scanner.go                # GitRepositoryScanner interface
│       ├── cloner.go                 # RepositoryCloner interface
│       ├── enricher.go               # Enricher interface
│       ├── embedding.go              # EmbeddingService interface
│       └── bm25.go                   # BM25Service
│
├── application/
│   ├── service/                      # Application/orchestration services
│   │   ├── code_search.go            # CodeSearchService (hybrid search orchestration)
│   │   ├── repository_sync.go        # RepositorySyncService
│   │   ├── repository_query.go       # RepositoryQueryService
│   │   ├── enrichment_query.go       # EnrichmentQueryService
│   │   ├── queue.go                  # QueueService
│   │   └── worker.go                 # IndexingWorkerService (internal, auto-started)
│   │
│   ├── handler/                      # Task handlers (command handlers)
│   │   ├── handler.go                # Handler interface, Registry
│   │   │
│   │   ├── repository/
│   │   │   ├── clone.go
│   │   │   ├── sync.go
│   │   │   └── delete.go
│   │   │
│   │   ├── commit/
│   │   │   └── scan.go
│   │   │
│   │   ├── indexing/
│   │   │   ├── extract_snippets.go
│   │   │   ├── create_bm25.go
│   │   │   └── create_embeddings.go
│   │   │
│   │   └── enrichment/
│   │       ├── create_summary.go
│   │       ├── commit_description.go
│   │       ├── architecture_discovery.go
│   │       ├── database_schema.go
│   │       ├── cookbook.go
│   │       ├── api_docs.go
│   │       ├── extract_examples.go
│   │       └── example_summary.go
│   │
│   └── dto/
│       ├── repository.go
│       ├── search.go
│       └── enrichment.go
│
├── infrastructure/
│   ├── persistence/                  # Database storage (GORM handles SQLite/Postgres)
│   │   ├── db.go                     # Database connection, GORM setup
│   │   ├── models.go                 # All GORM models (DB entities)
│   │   ├── mappers.go                # Domain <-> DB mappers
│   │   ├── query.go                  # QueryBuilder utilities
│   │   ├── repository_store.go       # RepositoryStore implementation
│   │   ├── commit_store.go           # CommitStore implementation
│   │   ├── branch_store.go           # BranchStore implementation
│   │   ├── tag_store.go              # TagStore implementation
│   │   ├── file_store.go             # FileStore implementation
│   │   ├── snippet_store.go          # SnippetStore implementation
│   │   ├── enrichment_store.go       # EnrichmentStore implementation
│   │   └── task_store.go             # TaskStore implementation
│   │
│   ├── search/                       # Search implementations
│   │   ├── bm25_sqlite.go            # SQLite FTS5 BM25
│   │   ├── bm25_postgres.go          # PostgreSQL native BM25
│   │   ├── bm25_vectorchord.go       # VectorChord BM25
│   │   ├── vector_sqlite.go          # SQLite vector (placeholder/future)
│   │   ├── vector_postgres.go        # PostgreSQL pgvector
│   │   └── vector_vectorchord.go     # VectorChord vector search
│   │
│   ├── provider/                     # AI providers (built-in only)
│   │   ├── provider.go               # TextGenerator, Embedder interfaces
│   │   ├── openai.go                 # OpenAI (text + embeddings)
│   │   └── anthropic.go              # Anthropic Claude (text only)
│   │
│   ├── git/                          # Git operations
│   │   ├── adapter.go                # Git adapter interface
│   │   ├── gogit.go                  # go-git implementation
│   │   ├── cloner.go                 # RepositoryCloner implementation
│   │   ├── scanner.go                # GitRepositoryScanner implementation
│   │   └── ignore.go                 # Ignore pattern matching
│   │
│   ├── slicing/                      # AST-based code extraction
│   │   ├── slicer.go                 # Main Slicer service
│   │   ├── config.go                 # LanguageConfig
│   │   ├── ast.go                    # AST utilities
│   │   ├── analyzer.go               # LanguageAnalyzer interface
│   │   └── language/
│   │       ├── python.go
│   │       ├── go.go
│   │       ├── javascript.go
│   │       ├── typescript.go
│   │       ├── java.go
│   │       ├── rust.go
│   │       ├── c.go
│   │       ├── cpp.go
│   │       └── csharp.go
│   │
│   ├── enricher/                     # Enrichment generation
│   │   ├── enricher.go               # Enricher implementation (uses TextGenerator)
│   │   ├── physical_architecture.go  # Docker Compose analysis
│   │   ├── cookbook_context.go       # README/package manifest parsing
│   │   └── example/
│   │       ├── discovery.go
│   │       ├── parser.go
│   │       └── code_block.go
│   │
│   ├── tracking/                     # Progress reporting
│   │   ├── tracker.go                # ProgressTracker implementation
│   │   ├── reporter.go               # Reporter interface
│   │   ├── logging.go                # LoggingReporter
│   │   └── db.go                     # DBReporter
│   │
│   └── api/                          # HTTP API (optional)
│       ├── server.go
│       ├── middleware/
│       │   ├── logging.go
│       │   ├── correlation.go
│       │   ├── auth.go
│       │   └── error.go
│       │
│       ├── v1/
│       │   ├── repositories.go
│       │   ├── commits.go
│       │   ├── search.go
│       │   ├── enrichments.go
│       │   └── queue.go
│       │
│       ├── jsonapi/
│       │   ├── response.go
│       │   └── serializer.go
│       │
│       └── dto/
│           ├── repository.go
│           ├── search.go
│           ├── enrichment.go
│           ├── commit.go
│           └── queue.go
│
└── cmd/                              # CLI application
    └── kodit/
        ├── main.go                   # Root command setup
        ├── serve.go                  # serve subcommand
        ├── stdio.go                  # stdio subcommand (MCP)
        └── version.go                # version subcommand
```

---

## Current to Target Mapping

| Current Location | Target Location | Notes |
|------------------|-----------------|-------|
| `internal/git/repo.go` | `domain/repository/repository.go` | |
| `internal/git/commit.go` | `domain/repository/commit.go` | |
| `internal/git/branch.go` | `domain/repository/branch.go` | |
| `internal/git/tag.go` | `domain/repository/tag.go` | |
| `internal/git/file.go` | `domain/repository/file.go` | |
| `internal/git/author.go` | `domain/repository/author.go` | |
| `internal/git/working_copy.go` | `domain/repository/working_copy.go` | |
| `internal/git/tracking_config.go` | `domain/repository/tracking_config.go` | |
| `internal/git/repository.go` (interfaces) | `domain/repository/store.go` | Rename to Store |
| `internal/git/scanner.go` | `domain/service/scanner.go` | Interface only |
| `internal/git/cloner.go` | `domain/service/cloner.go` | Interface only |
| `internal/git/adapter.go` | `infrastructure/git/adapter.go` | |
| `internal/git/gitadapter/gogit.go` | `infrastructure/git/gogit.go` | |
| `internal/git/ignore.go` | `infrastructure/git/ignore.go` | |
| `internal/git/postgres/*.go` | `infrastructure/persistence/*.go` | Combined |
| `internal/indexing/snippet.go` | `domain/snippet/snippet.go` | |
| `internal/indexing/commit_index.go` | `domain/snippet/index.go` | |
| `internal/indexing/repository.go` | `domain/snippet/store.go` | Split interfaces |
| `internal/indexing/bm25_service.go` | `domain/service/bm25.go` | |
| `internal/indexing/embedding_service.go` | `domain/service/embedding.go` | Interface only |
| `internal/indexing/slicer/*.go` | `infrastructure/slicing/*.go` | |
| `internal/indexing/bm25/*.go` | `infrastructure/search/bm25_*.go` | Combined |
| `internal/indexing/vector/*.go` | `infrastructure/search/vector_*.go` | Combined |
| `internal/indexing/postgres/*.go` | `infrastructure/persistence/*.go` | Combined |
| `internal/enrichment/enrichment.go` | `domain/enrichment/enrichment.go` | |
| `internal/enrichment/architecture.go` | `domain/enrichment/architecture.go` | |
| `internal/enrichment/development.go` | `domain/enrichment/development.go` | |
| `internal/enrichment/history.go` | `domain/enrichment/history.go` | |
| `internal/enrichment/usage.go` | `domain/enrichment/usage.go` | |
| `internal/enrichment/association.go` | `domain/enrichment/association.go` | |
| `internal/enrichment/repository.go` | `domain/enrichment/store.go` | Rename to Store |
| `internal/enrichment/enricher.go` | `infrastructure/enricher/enricher.go` | |
| `internal/enrichment/example/*.go` | `infrastructure/enricher/example/*.go` | |
| `internal/enrichment/postgres/*.go` | `infrastructure/persistence/*.go` | Combined |
| `internal/enrichment/physical_architecture.go` | `infrastructure/enricher/physical_architecture.go` | |
| `internal/enrichment/cookbook_context.go` | `infrastructure/enricher/cookbook_context.go` | |
| `internal/queue/task.go` | `domain/task/task.go` | |
| `internal/queue/status.go` | `domain/task/status.go` | |
| `internal/queue/operation.go` | `domain/task/operation.go` | |
| `internal/queue/repository.go` | `domain/task/store.go` | Rename to Store |
| `internal/queue/service.go` | `application/service/queue.go` | |
| `internal/queue/worker.go` | `application/service/worker.go` | |
| `internal/queue/handler.go` | `application/handler/handler.go` | |
| `internal/queue/registry.go` | `application/handler/handler.go` | Combine |
| `internal/queue/handler/*.go` | `application/handler/**/*.go` | Reorganise |
| `internal/queue/postgres/*.go` | `infrastructure/persistence/*.go` | Combined |
| `internal/tracking/*.go` | `domain/tracking/*.go` + `infrastructure/tracking/*.go` | Split |
| `internal/repository/query.go` | `application/service/repository_query.go` | |
| `internal/repository/sync.go` | `application/service/repository_sync.go` | |
| `internal/search/service.go` | `application/service/code_search.go` | |
| `internal/search/fusion_service.go` | `domain/search/fusion.go` | Domain logic |
| `internal/domain/value.go` | `domain/search/filters.go` + others | Split |
| `internal/domain/errors.go` | `errors.go` | Promote to root |
| `internal/config/*.go` | `options.go` | Functional options |
| `internal/log/*.go` | Keep internal or inline | |
| `internal/provider/*.go` | `infrastructure/provider/*.go` | |
| `internal/database/*.go` | `infrastructure/persistence/db.go` | Combine |
| `internal/api/*.go` | `infrastructure/api/*.go` | Move |
| `internal/mcp/*.go` | Inline in `cmd/kodit/stdio.go` | Remove separate package |
| `internal/factory/*.go` | Integrated into `kodit.go` | Absorb |
| `cmd/kodit/main.go` | `cmd/kodit/*.go` | Split into files |

---

## Public API Surface

### Root Package (`kodit`)

```go
package kodit

// Client is the main entry point for the kodit library.
// The background worker starts automatically on creation.
type Client struct {
    // private fields
}

// New creates a new Client with the given options.
// The background worker is started automatically.
func New(opts ...Option) (*Client, error)

// Close releases all resources and stops the background worker.
func (c *Client) Close() error

// Repositories returns the repository management interface.
func (c *Client) Repositories() Repositories

// Search performs a hybrid code search.
func (c *Client) Search(ctx context.Context, query string, opts ...SearchOption) (SearchResult, error)

// Enrichments returns the enrichment query interface.
func (c *Client) Enrichments() Enrichments

// Tasks returns the task queue interface.
func (c *Client) Tasks() Tasks

// API returns an HTTP server that can be started.
func (c *Client) API() APIServer
```

### Options (`options.go`)

```go
// Option configures the Client.
type Option func(*clientConfig)

// WithSQLite configures SQLite as the storage backend.
// BM25 uses FTS5, vector search uses the configured provider.
func WithSQLite(path string) Option

// WithPostgres configures PostgreSQL as the storage backend.
// Uses native PostgreSQL full-text search for BM25.
func WithPostgres(dsn string) Option

// WithPostgresPgvector configures PostgreSQL with pgvector extension.
func WithPostgresPgvector(dsn string) Option

// WithPostgresVectorchord configures PostgreSQL with VectorChord extension.
// VectorChord provides both BM25 and vector search.
func WithPostgresVectorchord(dsn string) Option

// WithOpenAI sets OpenAI as the AI provider (text + embeddings).
func WithOpenAI(apiKey string) Option

// WithOpenAIConfig sets OpenAI with custom configuration.
func WithOpenAIConfig(cfg OpenAIConfig) Option

// WithAnthropic sets Anthropic Claude as the text generation provider.
// Requires a separate embedding provider.
func WithAnthropic(apiKey string) Option

// WithDataDir sets the data directory for cloned repositories.
func WithDataDir(dir string) Option

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option

// WithAPIKey sets the API key for HTTP API authentication.
func WithAPIKey(key string) Option
```

### Search Options

```go
// SearchOption configures a search request.
type SearchOption func(*searchConfig)

// WithSemanticWeight sets the weight for semantic (vector) search (0-1).
func WithSemanticWeight(w float64) SearchOption

// WithLimit sets the maximum number of results.
func WithLimit(n int) SearchOption

// WithLanguages filters results by programming languages.
func WithLanguages(langs ...string) SearchOption

// WithRepositories filters results by repository IDs.
func WithRepositories(ids ...int64) SearchOption

// WithEnrichmentTypes includes specific enrichment types in results.
func WithEnrichmentTypes(types ...EnrichmentType) SearchOption
```

### Interfaces

```go
// Repositories provides repository management operations.
type Repositories interface {
    // Clone clones a repository and queues it for indexing.
    Clone(ctx context.Context, url string) (Repository, error)

    // Get retrieves a repository by ID.
    Get(ctx context.Context, id int64) (Repository, error)

    // List returns all repositories.
    List(ctx context.Context) ([]Repository, error)

    // Delete removes a repository and all associated data.
    Delete(ctx context.Context, id int64) error

    // Sync triggers re-indexing of a repository.
    Sync(ctx context.Context, id int64) error
}

// Enrichments provides enrichment query operations.
type Enrichments interface {
    // ForRepository returns enrichments for a repository.
    ForRepository(ctx context.Context, repoID int64, opts ...EnrichmentOption) ([]Enrichment, error)

    // ForCommit returns enrichments for a specific commit.
    ForCommit(ctx context.Context, repoID int64, commitSHA string, opts ...EnrichmentOption) ([]Enrichment, error)

    // Get retrieves a specific enrichment by ID.
    Get(ctx context.Context, id int64) (Enrichment, error)
}

// Tasks provides task queue operations.
type Tasks interface {
    // List returns pending tasks.
    List(ctx context.Context, opts ...TaskOption) ([]Task, error)

    // Get retrieves a task by ID.
    Get(ctx context.Context, id int64) (Task, error)

    // Cancel cancels a pending task.
    Cancel(ctx context.Context, id int64) error
}

// APIServer is an HTTP server.
type APIServer interface {
    // ListenAndServe starts the HTTP server.
    ListenAndServe(addr string) error

    // Shutdown gracefully shuts down the server.
    Shutdown(ctx context.Context) error
}
```

---

## Storage Backend Support

| Backend | BM25 | Vector | Notes |
|---------|------|--------|-------|
| SQLite | FTS5 | Future | Default for local development |
| PostgreSQL | Native FTS | pgvector | Requires pgvector extension |
| PostgreSQL + VectorChord | VectorChord | VectorChord | Single extension for both |

---

## Task List

### Phase 1: Create Domain Layer Structure

- [ ] 1.1 Create `domain/repository/` package
  - [ ] Move `internal/git/repo.go` → `domain/repository/repository.go`
  - [ ] Move `internal/git/commit.go` → `domain/repository/commit.go`
  - [ ] Move `internal/git/branch.go` → `domain/repository/branch.go`
  - [ ] Move `internal/git/tag.go` → `domain/repository/tag.go`
  - [ ] Move `internal/git/file.go` → `domain/repository/file.go`
  - [ ] Move `internal/git/author.go` → `domain/repository/author.go`
  - [ ] Move `internal/git/working_copy.go` → `domain/repository/working_copy.go`
  - [ ] Move `internal/git/tracking_config.go` → `domain/repository/tracking_config.go`
  - [ ] Create `domain/repository/store.go` from `internal/git/repository.go`

- [ ] 1.2 Create `domain/snippet/` package
  - [ ] Move `internal/indexing/snippet.go` → `domain/snippet/snippet.go`
  - [ ] Move `internal/indexing/commit_index.go` → `domain/snippet/index.go`
  - [ ] Create `domain/snippet/language.go` (extract from value.go)
  - [ ] Create `domain/snippet/store.go` from `internal/indexing/repository.go`

- [ ] 1.3 Create `domain/enrichment/` package
  - [ ] Move `internal/enrichment/enrichment.go` → `domain/enrichment/enrichment.go`
  - [ ] Move `internal/enrichment/association.go` → `domain/enrichment/association.go`
  - [ ] Move `internal/enrichment/development.go` → `domain/enrichment/development.go`
  - [ ] Move `internal/enrichment/architecture.go` → `domain/enrichment/architecture.go`
  - [ ] Move `internal/enrichment/history.go` → `domain/enrichment/history.go`
  - [ ] Move `internal/enrichment/usage.go` → `domain/enrichment/usage.go`
  - [ ] Create `domain/enrichment/store.go` from `internal/enrichment/repository.go`

- [ ] 1.4 Create `domain/task/` package
  - [ ] Move `internal/queue/task.go` → `domain/task/task.go`
  - [ ] Move `internal/queue/status.go` → `domain/task/status.go`
  - [ ] Move `internal/queue/operation.go` → `domain/task/operation.go`
  - [ ] Create `domain/task/store.go` from `internal/queue/repository.go`

- [ ] 1.5 Create `domain/search/` package
  - [ ] Create `domain/search/query.go` (extract from value.go)
  - [ ] Create `domain/search/result.go` (from search/service.go)
  - [ ] Create `domain/search/filters.go` (extract from value.go)
  - [ ] Move `internal/search/fusion_service.go` → `domain/search/fusion.go`
  - [ ] Create `domain/search/bm25.go` (interface from indexing/repository.go)
  - [ ] Create `domain/search/vector.go` (interface from indexing/repository.go)

- [ ] 1.6 Create `domain/tracking/` package
  - [ ] Move `internal/tracking/trackable.go` → `domain/tracking/trackable.go`
  - [ ] Move `internal/tracking/status.go` → `domain/tracking/status.go`
  - [ ] Move `internal/tracking/resolver.go` → `domain/tracking/resolution.go`

- [ ] 1.7 Create `domain/service/` package
  - [ ] Create `domain/service/scanner.go` (interface from git/scanner.go)
  - [ ] Create `domain/service/cloner.go` (interface from git/cloner.go)
  - [ ] Create `domain/service/enricher.go` (interface from enrichment/enricher.go)
  - [ ] Create `domain/service/embedding.go` (interface from indexing)
  - [ ] Move `internal/indexing/bm25_service.go` → `domain/service/bm25.go`

### Phase 2: Create Application Layer Structure

- [ ] 2.1 Create `application/service/` package
  - [ ] Move `internal/search/service.go` → `application/service/code_search.go`
  - [ ] Move `internal/repository/sync.go` → `application/service/repository_sync.go`
  - [ ] Move `internal/repository/query.go` → `application/service/repository_query.go`
  - [ ] Move `internal/enrichment/query.go` → `application/service/enrichment_query.go`
  - [ ] Move `internal/queue/service.go` → `application/service/queue.go`
  - [ ] Move `internal/queue/worker.go` → `application/service/worker.go`

- [ ] 2.2 Create `application/handler/` package
  - [ ] Move `internal/queue/handler.go` → `application/handler/handler.go`
  - [ ] Merge `internal/queue/registry.go` into `application/handler/handler.go`
  - [ ] Move `internal/queue/handler/clone_repository.go` → `application/handler/repository/clone.go`
  - [ ] Move `internal/queue/handler/sync_repository.go` → `application/handler/repository/sync.go`
  - [ ] Move `internal/queue/handler/delete_repository.go` → `application/handler/repository/delete.go`
  - [ ] Move `internal/queue/handler/scan_commit.go` → `application/handler/commit/scan.go`
  - [ ] Move `internal/queue/handler/extract_snippets.go` → `application/handler/indexing/extract_snippets.go`
  - [ ] Move `internal/queue/handler/create_bm25.go` → `application/handler/indexing/create_bm25.go`
  - [ ] Move `internal/queue/handler/create_embeddings.go` → `application/handler/indexing/create_embeddings.go`
  - [ ] Move `internal/queue/handler/enrichment/*.go` → `application/handler/enrichment/*.go`

- [ ] 2.3 Create `application/dto/` package
  - [ ] Extract DTOs from internal/repository into `application/dto/repository.go`
  - [ ] Extract DTOs from internal/search into `application/dto/search.go`
  - [ ] Extract DTOs from internal/enrichment into `application/dto/enrichment.go`

### Phase 3: Consolidate Infrastructure Layer

- [ ] 3.1 Create `infrastructure/persistence/` package
  - [ ] Create `infrastructure/persistence/db.go` (from internal/database)
  - [ ] Create `infrastructure/persistence/models.go` (combine all GORM entities)
  - [ ] Create `infrastructure/persistence/mappers.go` (combine all mappers)
  - [ ] Create `infrastructure/persistence/query.go` (from internal/database/query.go)
  - [ ] Create store implementations for each aggregate

- [ ] 3.2 Create `infrastructure/search/` package
  - [ ] Create `infrastructure/search/bm25_sqlite.go` (SQLite FTS5)
  - [ ] Create `infrastructure/search/bm25_postgres.go` (PostgreSQL native)
  - [ ] Move VectorChord BM25 → `infrastructure/search/bm25_vectorchord.go`
  - [ ] Create `infrastructure/search/vector_postgres.go` (pgvector)
  - [ ] Move VectorChord vector → `infrastructure/search/vector_vectorchord.go`

- [ ] 3.3 Create `infrastructure/provider/` package
  - [ ] Move `internal/provider/provider.go` → `infrastructure/provider/provider.go`
  - [ ] Move `internal/provider/openai.go` → `infrastructure/provider/openai.go`
  - [ ] Create `infrastructure/provider/anthropic.go`

- [ ] 3.4 Create `infrastructure/git/` package
  - [ ] Move `internal/git/adapter.go` → `infrastructure/git/adapter.go`
  - [ ] Move `internal/git/gitadapter/gogit.go` → `infrastructure/git/gogit.go`
  - [ ] Move `internal/git/cloner.go` → `infrastructure/git/cloner.go`
  - [ ] Move `internal/git/scanner.go` → `infrastructure/git/scanner.go`
  - [ ] Move `internal/git/ignore.go` → `infrastructure/git/ignore.go`

- [ ] 3.5 Create `infrastructure/slicing/` package
  - [ ] Move `internal/indexing/slicer/*.go` → `infrastructure/slicing/*.go`
  - [ ] Move `internal/indexing/slicer/analyzers/*.go` → `infrastructure/slicing/language/*.go`

- [ ] 3.6 Create `infrastructure/enricher/` package
  - [ ] Move `internal/enrichment/enricher.go` → `infrastructure/enricher/enricher.go`
  - [ ] Move `internal/enrichment/physical_architecture.go` → `infrastructure/enricher/physical_architecture.go`
  - [ ] Move `internal/enrichment/cookbook_context.go` → `infrastructure/enricher/cookbook_context.go`
  - [ ] Move `internal/enrichment/example/*.go` → `infrastructure/enricher/example/*.go`

- [ ] 3.7 Create `infrastructure/tracking/` package
  - [ ] Move `internal/tracking/tracker.go` → `infrastructure/tracking/tracker.go`
  - [ ] Move `internal/tracking/reporter.go` → `infrastructure/tracking/reporter.go`
  - [ ] Move `internal/tracking/logging_reporter.go` → `infrastructure/tracking/logging.go`
  - [ ] Move `internal/tracking/db_reporter.go` → `infrastructure/tracking/db.go`

- [ ] 3.8 Move `infrastructure/api/` package
  - [ ] Move `internal/api/*.go` → `infrastructure/api/*.go`

### Phase 4: Create Public API

- [ ] 4.1 Create root package files
  - [ ] Create `kodit.go` with `Client` type and `New()` constructor
  - [ ] Create `options.go` with functional options
  - [ ] Create `errors.go` with exported errors (promote from internal/domain)

- [ ] 4.2 Implement `Client` methods
  - [ ] Implement `Repositories()` method returning `Repositories` interface
  - [ ] Implement `Search()` method
  - [ ] Implement `Enrichments()` method returning `Enrichments` interface
  - [ ] Implement `Tasks()` method returning `Tasks` interface
  - [ ] Implement `API()` method returning `APIServer`
  - [ ] Implement `Close()` method (stops worker, closes DB)

- [ ] 4.3 Implement automatic worker startup
  - [ ] Start worker in `New()` after all dependencies are wired
  - [ ] Stop worker in `Close()`

- [ ] 4.4 Write integration tests for public API
  - [ ] Test `kodit.New()` with SQLite
  - [ ] Test `Repositories().Clone()` and `List()`
  - [ ] Test `Search()` with various options
  - [ ] Test `API().ListenAndServe()`

### Phase 5: Simplify CLI

- [ ] 5.1 Split `cmd/kodit/main.go` into separate files
  - [ ] Create `cmd/kodit/main.go` (root command only)
  - [ ] Create `cmd/kodit/serve.go` (serve subcommand)
  - [ ] Create `cmd/kodit/stdio.go` (stdio/MCP subcommand)
  - [ ] Create `cmd/kodit/version.go` (version subcommand)

- [ ] 5.2 Simplify CLI to use library
  - [ ] Update serve.go to use `kodit.New()` and `client.API().ListenAndServe()`
  - [ ] Update stdio.go to use `kodit.New()` for MCP (inline MCP code)
  - [ ] Remove `internal/factory/` package

### Phase 6: Cleanup

- [ ] 6.1 Remove old internal packages
  - [ ] Remove `internal/git/` (replaced by domain/repository + infrastructure/git)
  - [ ] Remove `internal/indexing/` (replaced by domain/snippet + infrastructure)
  - [ ] Remove `internal/enrichment/` (replaced by domain/enrichment + infrastructure)
  - [ ] Remove `internal/queue/` (replaced by domain/task + application)
  - [ ] Remove `internal/repository/` (replaced by application/service)
  - [ ] Remove `internal/search/` (replaced by domain/search + application)
  - [ ] Remove `internal/tracking/` (replaced by domain + infrastructure)
  - [ ] Remove `internal/domain/` (replaced by domain/)
  - [ ] Remove `internal/database/` (replaced by infrastructure/persistence)
  - [ ] Remove `internal/provider/` (replaced by infrastructure/provider)
  - [ ] Remove `internal/mcp/` (inlined into CLI)

- [ ] 6.2 Update all imports
  - [ ] Update all test files
  - [ ] Verify all tests pass

- [ ] 6.3 Documentation
  - [ ] Add godoc to all public types
  - [ ] Update README with library usage
  - [ ] Create `examples/` directory with usage examples

---

## Compatibility Notes

### Database Schema

No schema changes required. The refactored library uses the same GORM models and migrations.

### API Endpoints

HTTP API remains 100% compatible with Python implementation. JSON:API format preserved.

### Task Operations

Task operations and payloads remain compatible for queue processing.

---

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| MCP location | Inline in CLI | Not a separate package, just implementation detail |
| Persistence structure | Single package | GORM handles SQLite/Postgres transparently |
| Test storage | In-memory SQLite | Simple, fast, no external dependencies |
| Worker lifecycle | Auto-start | Hidden complexity from library users |
| Config mechanism | Functional options + env vars | Simple, composable, no config files |
| AI providers | Built-in only | Simplicity first, extend later if needed |

---

## Success Criteria

The refactor is complete when:

- [ ] All existing tests pass
- [ ] Library can be used without starting any servers
- [ ] HTTP API can be optionally started via `client.API()`
- [ ] CLI works exactly as before
- [ ] SQLite, PostgreSQL, pgvector, and VectorChord all supported
- [ ] Worker starts automatically and stops on `Close()`
- [ ] At least one example program demonstrates embedded usage
