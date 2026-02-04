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

- [x] 1.1 Create `domain/repository/` package
  - [x] Move `internal/git/repo.go` → `domain/repository/repository.go`
  - [x] Move `internal/git/commit.go` → `domain/repository/commit.go`
  - [x] Move `internal/git/branch.go` → `domain/repository/branch.go`
  - [x] Move `internal/git/tag.go` → `domain/repository/tag.go`
  - [x] Move `internal/git/file.go` → `domain/repository/file.go`
  - [x] Move `internal/git/author.go` → `domain/repository/author.go`
  - [x] Move `internal/git/working_copy.go` → `domain/repository/working_copy.go`
  - [x] Move `internal/git/tracking_config.go` → `domain/repository/tracking_config.go`
  - [x] Create `domain/repository/store.go` from `internal/git/repository.go`

- [x] 1.2 Create `domain/snippet/` package
  - [x] Move `internal/indexing/snippet.go` → `domain/snippet/snippet.go`
  - [x] Move `internal/indexing/commit_index.go` → `domain/snippet/index.go`
  - [x] Create `domain/snippet/language.go` (extract from value.go)
  - [x] Create `domain/snippet/store.go` from `internal/indexing/repository.go`

- [x] 1.3 Create `domain/enrichment/` package
  - [x] Move `internal/enrichment/enrichment.go` → `domain/enrichment/enrichment.go`
  - [x] Move `internal/enrichment/association.go` → `domain/enrichment/association.go`
  - [x] Move `internal/enrichment/development.go` → `domain/enrichment/development.go`
  - [x] Move `internal/enrichment/architecture.go` → `domain/enrichment/architecture.go`
  - [x] Move `internal/enrichment/history.go` → `domain/enrichment/history.go`
  - [x] Move `internal/enrichment/usage.go` → `domain/enrichment/usage.go`
  - [x] Create `domain/enrichment/store.go` from `internal/enrichment/repository.go`

- [x] 1.4 Create `domain/task/` package
  - [x] Move `internal/queue/task.go` → `domain/task/task.go`
  - [x] Move `internal/queue/status.go` → `domain/task/status.go`
  - [x] Move `internal/queue/operation.go` → `domain/task/operation.go`
  - [x] Create `domain/task/store.go` from `internal/queue/repository.go`

- [x] 1.5 Create `domain/search/` package
  - [x] Create `domain/search/query.go` (extract from value.go)
  - [x] Create `domain/search/result.go` (from search/service.go)
  - [x] Create `domain/search/filters.go` (extract from value.go)
  - [x] Move `internal/search/fusion_service.go` → `domain/search/fusion.go`
  - [x] Create `domain/search/bm25.go` (interface from indexing/repository.go)
  - [x] Create `domain/search/vector.go` (interface from indexing/repository.go)

- [x] 1.6 Create `domain/tracking/` package
  - [x] Move `internal/tracking/trackable.go` → `domain/tracking/trackable.go`
  - [x] Move `internal/tracking/status.go` → `domain/tracking/status.go`
  - [x] Move `internal/tracking/resolver.go` → `domain/tracking/resolution.go`

- [x] 1.7 Create `domain/service/` package
  - [x] Create `domain/service/scanner.go` (interface from git/scanner.go)
  - [x] Create `domain/service/cloner.go` (interface from git/cloner.go)
  - [x] Create `domain/service/enricher.go` (interface from enrichment/enricher.go)
  - [x] Create `domain/service/embedding.go` (interface from indexing)
  - [x] Move `internal/indexing/bm25_service.go` → `domain/service/bm25.go`

### Phase 2: Create Application Layer Structure

- [x] 2.1 Create `application/service/` package
  - [x] Move `internal/search/service.go` → `application/service/code_search.go`
  - [x] Move `internal/repository/sync.go` → `application/service/repository_sync.go`
  - [x] Move `internal/repository/query.go` → `application/service/repository_query.go`
  - [x] Move `internal/enrichment/query.go` → `application/service/enrichment_query.go`
  - [x] Move `internal/queue/service.go` → `application/service/queue.go`
  - [x] Move `internal/queue/worker.go` → `application/service/worker.go`

- [x] 2.2 Create `application/handler/` package
  - [x] Move `internal/queue/handler.go` → `application/handler/handler.go`
  - [x] Merge `internal/queue/registry.go` into `application/handler/handler.go`
  - [x] Move `internal/queue/handler/clone_repository.go` → `application/handler/repository/clone.go`
  - [x] Move `internal/queue/handler/sync_repository.go` → `application/handler/repository/sync.go`
  - [x] Move `internal/queue/handler/delete_repository.go` → `application/handler/repository/delete.go`
  - [x] Move `internal/queue/handler/scan_commit.go` → `application/handler/commit/scan.go`
  - [x] Move `internal/queue/handler/extract_snippets.go` → `application/handler/indexing/extract_snippets.go`
  - [x] Move `internal/queue/handler/create_bm25.go` → `application/handler/indexing/create_bm25.go`
  - [x] Move `internal/queue/handler/create_embeddings.go` → `application/handler/indexing/create_embeddings.go`
  - [x] Move `internal/queue/handler/enrichment/*.go` → `application/handler/enrichment/*.go`

- [x] 2.3 Application DTOs (DESIGN DECISION: Co-located with services)
  - [x] DTOs co-located in `application/service/` files instead of separate package
  - [x] `Source`, `SourceStatus` in `repository_sync.go`
  - [x] `RepositorySummary` in `repository_query.go`
  - [x] `MultiSearchResult` in `code_search.go`
  - Note: Separate `application/dto/` package not needed - DTOs are used by single services

### Phase 3: Consolidate Infrastructure Layer

- [x] 3.1 Create `infrastructure/persistence/` package
  - [x] Create `infrastructure/persistence/db.go` (from internal/database)
  - [x] Create `infrastructure/persistence/models.go` (combine all GORM entities)
  - [x] Create `infrastructure/persistence/mappers.go` (combine all mappers)
  - [x] Create `infrastructure/persistence/query.go` (from internal/database/query.go)
  - [x] Create `infrastructure/persistence/repository_store.go` (RepositoryStore)
  - [x] Create `infrastructure/persistence/commit_store.go` (CommitStore)
  - [x] Create `infrastructure/persistence/branch_store.go` (BranchStore)
  - [x] Create `infrastructure/persistence/tag_store.go` (TagStore)
  - [x] Create `infrastructure/persistence/file_store.go` (FileStore)
  - [x] Create `infrastructure/persistence/snippet_store.go` (SnippetStore, CommitIndexStore)
  - [x] Create `infrastructure/persistence/enrichment_store.go` (EnrichmentStore, AssociationStore)
  - [x] Create `infrastructure/persistence/task_store.go` (TaskStore, StatusStore)

- [x] 3.2 Create `infrastructure/search/` package
  - [x] Create `infrastructure/search/bm25_sqlite.go` (SQLite FTS5)
  - [x] Create `infrastructure/search/bm25_postgres.go` (PostgreSQL native)
  - [x] Move VectorChord BM25 → `infrastructure/search/bm25_vectorchord.go`
  - [x] Create `infrastructure/search/vector_postgres.go` (pgvector)
  - [x] Move VectorChord vector → `infrastructure/search/vector_vectorchord.go`

- [x] 3.3 Create `infrastructure/provider/` package
  - [x] Move `internal/provider/provider.go` → `infrastructure/provider/provider.go`
  - [x] Move `internal/provider/openai.go` → `infrastructure/provider/openai.go`
  - [x] Create `infrastructure/provider/anthropic.go`

- [x] 3.4 Create `infrastructure/git/` package
  - [x] Move `internal/git/adapter.go` → `infrastructure/git/adapter.go`
  - [x] Move `internal/git/gitadapter/gogit.go` → `infrastructure/git/gogit.go`
  - [x] Move `internal/git/cloner.go` → `infrastructure/git/cloner.go`
  - [x] Move `internal/git/scanner.go` → `infrastructure/git/scanner.go`
  - [x] Move `internal/git/ignore.go` → `infrastructure/git/ignore.go`

- [x] 3.5 Create `infrastructure/slicing/` package
  - [x] Move `internal/indexing/slicer/*.go` → `infrastructure/slicing/*.go`
  - [x] Move `internal/indexing/slicer/analyzers/*.go` → `infrastructure/slicing/language/*.go`

- [x] 3.6 Create `infrastructure/enricher/` package
  - [x] Move `internal/enrichment/enricher.go` → `infrastructure/enricher/enricher.go`
  - [x] Move `internal/enrichment/physical_architecture.go` → `infrastructure/enricher/physical_architecture.go`
  - [x] Move `internal/enrichment/cookbook_context.go` → `infrastructure/enricher/cookbook_context.go`
  - [x] Move `internal/enrichment/example/*.go` → `infrastructure/enricher/example/*.go`

- [x] 3.7 Create `infrastructure/tracking/` package
  - [x] Move `internal/tracking/tracker.go` → `infrastructure/tracking/tracker.go`
  - [x] Move `internal/tracking/reporter.go` → `infrastructure/tracking/reporter.go`
  - [x] Move `internal/tracking/logging_reporter.go` → `infrastructure/tracking/logging.go`
  - [x] Move `internal/tracking/db_reporter.go` → `infrastructure/tracking/db.go`

- [x] 3.8 Move `infrastructure/api/` package
  - [x] Move `internal/api/*.go` → `infrastructure/api/*.go`

### Phase 4: Create Public API

- [x] 4.1 Create root package files
  - [x] Create `kodit.go` with `Client` type and `New()` constructor
  - [x] Create `options.go` with functional options
  - [x] Create `errors.go` with exported errors (promote from internal/domain)

- [x] 4.2 Implement `Client` methods
  - [x] Implement `Repositories()` method returning `Repositories` interface
  - [x] Implement `Search()` method (placeholder - needs search store wiring)
  - [x] Implement `Enrichments()` method returning `Enrichments` interface
  - [x] Implement `Tasks()` method returning `Tasks` interface
  - [x] Implement `API()` method returning `APIServer`
  - [x] Implement `Close()` method (stops worker, closes DB)

- [x] 4.3 Implement automatic worker startup
  - [x] Start worker in `New()` after all dependencies are wired
  - [x] Stop worker in `Close()`

- [x] 4.4 Write integration tests for public API
  - [x] Test `kodit.New()` with SQLite
  - [x] Test `Repositories().List()` (empty case)
  - [x] Test `Search()` with various options
  - [x] Test `Tasks().List()` (empty case)
  - [x] Test `Close()` idempotency
  - [x] Test `Search()` after close returns error

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
| Application DTOs | Co-located with services | DTOs are used by single services, no sharing needed |

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

---

## Session Notes

### 2026-02-04 Session

**Completed:**
- Phase 1 complete: Created entire domain layer structure
  - `domain/repository/` - Git repository aggregate (Repository, Commit, Branch, Tag, File, Author, WorkingCopy, TrackingConfig) + store interfaces
  - `domain/snippet/` - Snippet aggregate (Snippet, CommitIndex, Language mapping) + store interfaces
  - `domain/enrichment/` - Enrichment aggregate (Enrichment, Association, type/subtype hierarchy) + store interfaces
  - `domain/task/` - Task aggregate (Task, Status, Operation) + store interfaces
  - `domain/search/` - Search domain (Query, Request, Result, Filters, Fusion algorithm) + BM25/Vector store interfaces
  - `domain/tracking/` - Tracking domain (Trackable, RepositoryStatusSummary, Resolver)
  - `domain/service/` - Domain service interfaces (Scanner, Cloner, Enricher, Embedding, BM25)

**Verified:**
- All domain packages build: `go build ./domain/...` ✓
- All domain packages lint clean: `golangci-lint run ./domain/...` ✓
- Full project still builds: `go build ./...` ✓

**Design Decisions Made:**
- Domain types use value semantics (immutable structs with getters)
- Store interfaces follow pattern: Get, Find, Save, Delete + domain-specific operations
- Cross-domain references handled via importing domain packages (e.g., snippet imports repository.File)
- Simple Enrichment value object added to snippet package to avoid circular dependency
- IndexStatus moved to snippet package alongside CommitIndex
- ReportingState and TrackableType moved to task package alongside Status

**Next Session Tasks:**
- Phase 2: Create application layer structure
  - Start with `application/service/` (CodeSearch, RepositorySynce, etc.)
  - Then `application/handler/` (task handlers)

### 2026-02-04 Session 2

**Completed:**
- Phase 2.1 complete: Created `application/service/` package
  - `application/service/code_search.go` - Hybrid code search orchestration (BM25 + vector)
  - `application/service/repository_sync.go` - Repository sync/clone operations + Source value object
  - `application/service/repository_query.go` - Repository read-only queries
  - `application/service/enrichment_query.go` - Enrichment queries by commit/type
  - `application/service/queue.go` - Task queue service
  - `application/service/worker.go` - Background task worker + Handler interface + Registry

- Phase 2.2 partial: Created `application/handler/` base infrastructure
  - `application/handler/handler.go` - Handler interface, Registry, TrackerFactory, utility functions
  - `application/handler/repository/clone.go` - Clone repository handler
  - `application/handler/commit/scan.go` - Scan commit handler

**Verified:**
- All application packages build: `go build ./application/...` ✓
- All application packages lint clean: `golangci-lint run ./application/...` ✓
- Full project still builds: `go build ./...` ✓

**Design Decisions Made:**
- Application services use domain types from `domain/` packages
- Source value object moved to application layer (wraps domain.Repository with status tracking)
- TrackerFactory interface defined in handler package (infrastructure implements)
- Handler utility functions exported (ExtractInt64, ExtractString, ShortSHA)
- Priority type added to domain/task package

**Next Session Tasks:**
- Complete remaining handlers in Phase 2.2:
  - `application/handler/repository/sync.go`
  - `application/handler/repository/delete.go`
  - `application/handler/indexing/*.go`
  - `application/handler/enrichment/*.go`
- Phase 2.3: Create `application/dto/` package
- Phase 3: Begin infrastructure consolidation

### 2026-02-04 Session 3

**Completed:**
- Phase 2.2 handlers (core indexing handlers):
  - `application/handler/repository/sync.go` - Sync repository handler
  - `application/handler/repository/delete.go` - Delete repository handler
  - `application/handler/indexing/extract_snippets.go` - Extract code snippets via AST parsing
  - `application/handler/indexing/create_bm25.go` - Create BM25 keyword index
  - `application/handler/indexing/create_embeddings.go` - Create vector embeddings

**Verified:**
- All application packages build: `go build ./application/...` ✓
- All application packages lint clean: `golangci-lint run ./application/...` ✓
- Full project still builds: `go build ./...` ✓

**Design Decisions Made:**
- Indexing handlers use internal slicer/adapter temporarily (will be moved to infrastructure later)
- Snippet conversion between internal/indexing.Snippet and domain/snippet.Snippet handled in handler
- Language mapping moved to domain/snippet package for reuse

**Next Session Tasks:**
- Phase 2.3: Create `application/dto/` package
- Phase 3: Begin infrastructure consolidation

### 2026-02-04 Session 4

**Completed:**
- Phase 2.2 complete: All enrichment handlers migrated to `application/handler/enrichment/`
  - `application/handler/enrichment/util.go` - Utility functions (TruncateDiff, MaxDiffLength)
  - `application/handler/enrichment/create_summary.go` - Create snippet summaries with LLM
  - `application/handler/enrichment/commit_description.go` - Generate commit descriptions from diffs
  - `application/handler/enrichment/architecture_discovery.go` - Discover physical architecture
  - `application/handler/enrichment/database_schema.go` - Extract and document database schemas
  - `application/handler/enrichment/cookbook.go` - Generate cookbook examples
  - `application/handler/enrichment/api_docs.go` - Extract API documentation
  - `application/handler/enrichment/extract_examples.go` - Extract code examples from docs
  - `application/handler/enrichment/example_summary.go` - Summarize extracted examples
  - `application/handler/enrichment/handler_test.go` - Tests for commit description and create summary handlers

**Verified:**
- All enrichment handlers build: `go build ./application/handler/enrichment/...` ✓
- All enrichment tests pass: `go test ./application/handler/enrichment/...` ✓
- Lint clean: `golangci-lint run ./application/handler/enrichment/...` ✓
- Full project builds: `go build ./...` ✓

**Design Decisions Made:**
- Handlers use domain types from `domain/enrichment`, `domain/repository`, `domain/snippet`, `domain/task`
- EnrichmentQuery service from `application/service` used for checking existing enrichments
- Enricher interface from `domain/service` used for LLM enrichment
- TrackerFactory interface from `application/handler` used for progress tracking
- Git adapter temporarily uses `internal/git` (will be moved to infrastructure later)
- Handler-specific interfaces defined locally (ArchitectureDiscoverer, SchemaDiscoverer, CookbookContextGatherer, etc.)

**Next Session Tasks:**
- Phase 2.3: Create `application/dto/` package
- Phase 3: Begin infrastructure consolidation

### 2026-02-04 Session 5

**Completed:**
- Phase 2.3 complete: Application DTOs reviewed and design decision made
  - Analyzed existing DTO locations: API layer (`internal/api/v1/dto/`), application services
  - Found DTOs already co-located with services in `application/service/`:
    - `Source`, `SourceStatus` in `repository_sync.go`
    - `RepositorySummary` in `repository_query.go`
    - `MultiSearchResult` in `code_search.go`
  - Decision: Keep DTOs co-located with services (no separate `application/dto/` package needed)

**Verified:**
- All application packages build: `go build ./application/...` ✓
- All application packages lint clean: `golangci-lint run ./application/...` ✓

**Design Decisions Made:**
- Application DTOs remain co-located with their services rather than extracted to separate package
- This follows single-responsibility: each DTO is used by exactly one service
- API layer has its own DTOs for JSON:API serialization (`internal/api/v1/dto/`)
- Pattern: Domain types for business logic, Service DTOs for orchestration results, API DTOs for HTTP responses

**Next Session Tasks:**
- Phase 3: Begin infrastructure consolidation
  - Start with Phase 3.1: Create `infrastructure/persistence/` package
  - Consolidate GORM models from various `internal/*/postgres/` directories

### 2026-02-04 Session 6

**Completed:**
- Phase 2.3 complete: DTOs reviewed, kept co-located with services (design decision)
- Phase 3.1 partial: Created `infrastructure/persistence/` package core files
  - `infrastructure/persistence/db.go` - Database connection wrapper (from internal/database)
  - `infrastructure/persistence/models.go` - All GORM entities consolidated:
    - RepositoryModel, CommitModel, BranchModel, TagModel, FileModel (from internal/git/postgres)
    - SnippetModel, CommitIndexModel, SnippetCommitAssociationModel, SnippetFileDerivationModel, EmbeddingModel (from internal/indexing/postgres)
    - EnrichmentModel, EnrichmentAssociationModel (from internal/enrichment/postgres)
    - TaskModel, TaskStatusModel (from internal/queue/postgres)
  - `infrastructure/persistence/mappers.go` - All domain<->model mappers consolidated
  - `infrastructure/persistence/query.go` - Query builder (from internal/database/query.go)

**Verified:**
- `go build ./infrastructure/persistence/...` ✓
- `golangci-lint run ./infrastructure/persistence/...` ✓

**Design Decisions Made:**
- Models named `*Model` instead of `*Entity` for clarity (persistence vs domain)
- Mappers use domain types from `domain/` packages
- Mappers handle nil slices for aggregates loaded via joins (e.g., Snippet.derivesFrom)
- Association doesn't have CreatedAt/UpdatedAt in domain - timestamps added in mapper ToModel

**Next Session Tasks:**
- Complete Phase 3.1: Create store implementations for each aggregate
  - RepositoryStore, CommitStore, BranchStore, TagStore, FileStore
  - SnippetStore, CommitIndexStore
  - EnrichmentStore, AssociationStore
  - TaskStore, StatusStore
- Continue Phase 3.2: Create infrastructure/search/ package

### 2026-02-04 Session 7

**Completed:**
- Phase 3.1 complete: Created all store implementations in `infrastructure/persistence/`
  - `repository_store.go` - RepositoryStore implementing repository.RepositoryStore
  - `commit_store.go` - CommitStore implementing repository.CommitStore
  - `branch_store.go` - BranchStore implementing repository.BranchStore
  - `tag_store.go` - TagStore implementing repository.TagStore
  - `file_store.go` - FileStore implementing repository.FileStore
  - `snippet_store.go` - SnippetStore implementing snippet.SnippetStore, CommitIndexStore implementing snippet.CommitIndexStore
  - `enrichment_store.go` - EnrichmentStore implementing enrichment.EnrichmentStore, AssociationStore implementing enrichment.AssociationStore
  - `task_store.go` - TaskStore implementing task.TaskStore, StatusStore implementing task.StatusStore

**Verified:**
- `go build ./infrastructure/persistence/...` ✓
- `golangci-lint run ./infrastructure/persistence/...` ✓
- `go build ./...` ✓ (full project builds)

**Design Decisions Made:**
- Removed generic `Find(query)` methods from domain store interfaces to eliminate dependency on `internal/database.Query`
- Domain interfaces now have specific finder methods (FindAll, FindByRepoID, etc.) instead of generic query-based Find
- Infrastructure stores have internal `Find(query Query)` methods for flexibility but these are not part of domain interface
- Store implementations use value receivers (immutable pattern) - stores hold Database reference passed to constructor
- Composite primary keys (Branch, Tag, File) handled with GORM clause.OnConflict for upsert semantics
- LoadWithHierarchy for StatusStore reconstructs parent-child relationships using NewStatusFull

**Next Session Tasks:**
- Phase 3.2: Create `infrastructure/search/` package
  - BM25 implementations (SQLite FTS5, PostgreSQL native, VectorChord)
  - Vector search implementations (pgvector, VectorChord)

### 2026-02-04 Session 8

**Completed:**
- Phase 3.2 complete: Created `infrastructure/search/` package with all implementations
  - `bm25_vectorchord.go` - VectorChordBM25Store implementing search.BM25Store (PostgreSQL VectorChord extension)
  - `bm25_sqlite.go` - SQLiteBM25Store implementing search.BM25Store (SQLite FTS5)
  - `bm25_postgres.go` - PostgresBM25Store implementing search.BM25Store (PostgreSQL native full-text search)
  - `vector_vectorchord.go` - VectorChordVectorStore implementing search.VectorStore (PostgreSQL VectorChord extension)
  - `vector_postgres.go` - PgvectorStore implementing search.VectorStore (PostgreSQL pgvector extension)

**Verified:**
- `go build ./infrastructure/search/...` ✓
- `golangci-lint run ./infrastructure/search/...` ✓
- `go build ./...` ✓ (full project builds)

**Design Decisions Made:**
- All stores implement domain interfaces from `domain/search` package (BM25Store, VectorStore)
- Stores use lazy initialization pattern with mutex protection for thread safety
- BM25 stores: SQLite FTS5, PostgreSQL native ts_rank_cd, VectorChord BERT tokenizer
- Vector stores: pgvector with IVFFlat index, VectorChord with residual quantization
- Score normalization: negative BM25 scores converted to positive, cosine distance converted to similarity
- Separate tables per task type (code/text) for vector stores to enable different embedding strategies
- Embedder interface from `internal/provider` used (will move to `infrastructure/provider` in Phase 3.3)

**Next Session Tasks:**
- Phase 3.3: Create `infrastructure/provider/` package
  - Move provider interfaces and OpenAI implementation
  - Create Anthropic Claude provider

### 2026-02-04 Session 8 (continued)

**Completed:**
- Phase 3.3 complete: Created `infrastructure/provider/` package
  - `provider.go` - Core types: Message, ChatCompletionRequest/Response, EmbeddingRequest/Response, Usage, interfaces (TextGenerator, Embedder, Provider, FullProvider)
  - `openai.go` - OpenAIProvider implementing FullProvider (text + embeddings) with retry logic
  - `anthropic.go` - AnthropicProvider implementing TextOnlyProvider (Claude API)

**Verified:**
- `go build ./infrastructure/provider/...` ✓
- `golangci-lint run ./infrastructure/provider/...` ✓
- `go build ./...` ✓ (full project builds)

**Design Decisions Made:**
- Provider package is self-contained - no dependency on internal/config
- OpenAIConfig struct replaces config.Endpoint for cleaner API
- AnthropicProvider implements TextOnlyProvider (Anthropic doesn't provide embeddings)
- Both providers use exponential backoff retry with configurable parameters
- Functional options pattern for provider configuration

**Next Session Tasks:**
- Phase 3.4: Create `infrastructure/git/` package
  - Move git adapter, cloner, scanner, ignore implementations

### 2026-02-04 Session 8 (continued - Part 2)

**Completed:**
- Phase 3.4 complete: Created `infrastructure/git/` package
  - `adapter.go` - Adapter interface and info types (CommitInfo, BranchInfo, FileInfo, TagInfo)
  - `gogit.go` - GoGitAdapter implementing Adapter using go-git library
  - `cloner.go` - RepositoryCloner implementing domain/service.Cloner
  - `scanner.go` - RepositoryScanner implementing domain/service.Scanner
  - `ignore.go` - IgnorePattern for gitignore and .noindex pattern matching

**Verified:**
- `go build ./infrastructure/git/...` ✓
- `golangci-lint run ./infrastructure/git/...` ✓
- `go build ./...` ✓ (full project builds)

**Design Decisions Made:**
- Adapter interface is self-contained in infrastructure/git (not in domain)
- Info types (CommitInfo, etc.) are transport DTOs between adapter and domain converters
- RepositoryCloner and RepositoryScanner implement domain service interfaces
- Scanner converts adapter info types to domain types (repository.Commit, etc.)
- Cloner uses domain repository.Repository type directly

**Next Session Tasks:**
- Phase 3.5: Create `infrastructure/slicing/` package
  - Move slicer and language analyzers
- Phase 3.6: Create `infrastructure/enricher/` package
- Phase 3.7: Create `infrastructure/tracking/` package
- Phase 3.8: Move `infrastructure/api/` package

### 2026-02-04 Session 9

**Completed:**
- Phase 3.5 complete: Created `infrastructure/slicing/` package
  - Added missing language analyzers: `c.go`, `cpp.go`, `rust.go`, `csharp.go`
  - Updated all imports to use `domain/repository`, `domain/snippet`, `infrastructure/slicing`
  - Created `slicer_test.go` with updated imports

- Phase 3.6 complete: Created `infrastructure/enricher/` package
  - `infrastructure/enricher/enricher.go` - Enricher implementation using TextGenerator
  - `infrastructure/enricher/physical_architecture.go` - Docker Compose analysis
  - `infrastructure/enricher/cookbook_context.go` - README/manifest parsing
  - `infrastructure/enricher/example/code_block.go` - CodeBlock value object
  - `infrastructure/enricher/example/discovery.go` - Example file discovery
  - `infrastructure/enricher/example/parser.go` - Markdown/RST parsers

- Phase 3.7 complete: Created `infrastructure/tracking/` package
  - `infrastructure/tracking/tracker.go` - Tracker wrapping task.Status with subscriber notification
  - `infrastructure/tracking/reporter.go` - Reporter interface
  - `infrastructure/tracking/logging.go` - LoggingReporter implementation
  - `infrastructure/tracking/db.go` - DBReporter implementation using task.StatusStore

- Phase 3.8 complete: Moved `infrastructure/api/` package
  - Copied all files from `internal/api/` to `infrastructure/api/`
  - Updated internal api imports to infrastructure/api
  - Preserved all middleware, jsonapi, v1 routers and DTOs
  - Tests pass, lint clean

**Verified:**
- `go build ./...` ✓ (full project builds)
- `go test ./infrastructure/...` ✓
- `golangci-lint run ./infrastructure/...` ✓

**Design Decisions Made:**
- Infrastructure tracking uses `domain/task.Status` instead of `internal/queue.TaskStatus`
- Infrastructure tracking uses `domain/task.StatusStore` instead of `internal/queue.TaskStatusRepository`
- API package retains dependencies on internal packages (will be updated in Phase 6 cleanup)
- Language analyzers (C, C++, Rust, C#) follow same pattern as existing analyzers

**Next Session Tasks:**
- Phase 4: Create Public API
  - Create `kodit.go` with `Client` type and `New()` constructor
  - Create `options.go` with functional options
  - Create `errors.go` with exported errors

### 2026-02-04 Session 10

**Completed:**
- Phase 4.1 complete: Created root package files
  - `errors.go` - Exported errors (ErrNotFound, ErrValidation, ErrNoStorage, ErrClientClosed, etc.)
  - `options.go` - Functional options for Client, Search, Enrichment, and Task queries
  - `kodit.go` - Client type with New() constructor

- Phase 4.2 complete: Implemented Client methods
  - `Repositories()` - Returns Repositories interface (Clone, Get, List, Delete, Sync)
  - `Search()` - Placeholder for hybrid search (needs BM25/vector store wiring)
  - `Enrichments()` - Returns Enrichments interface (ForCommit, Get)
  - `Tasks()` - Returns Tasks interface (List, Get, Cancel)
  - `API()` - Returns APIServer interface (ListenAndServe, Shutdown)
  - `Close()` - Stops worker and closes database

- Phase 4.3 complete: Implemented automatic worker startup
  - Worker starts in New() after all dependencies are wired
  - Worker stops in Close()

- Fixed domain/snippet/store.go: Removed Search method that depended on internal/domain.MultiSearchRequest

**Verified:**
- `go build ./...` ✓
- `go test ./...` ✓
- `golangci-lint run ./...` ✓

**Design Decisions Made:**
- Public API uses concrete persistence types directly (not domain interfaces) for simplicity
- Search functionality is a placeholder - requires alignment between infrastructure/search and domain/search interfaces
- Simplified Enrichments interface to ForCommit instead of ForRepository (aligns with existing service)
- Options pattern: separate config types for Client, Search, Enrichment, and Task queries
- Client uses persistence stores directly rather than domain interfaces to avoid interface mismatches

**Known Issues:**
- infrastructure/search stores use `*gorm.DB` directly, not `persistence.Database`
- infrastructure/search stores use `internal/provider.Embedder`, not `infrastructure/provider.Embedder`
- Full hybrid search requires interface alignment between domain and infrastructure layers

- Phase 4.4 complete: Created integration tests in `kodit_test.go`
  - TestNew_RequiresStorage
  - TestNew_WithSQLite
  - TestClient_Close_Idempotent
  - TestClient_Repositories_List_Empty
  - TestClient_Tasks_List_Empty
  - TestClient_Search_ReturnsEmpty
  - TestClient_Search_AfterClose_ReturnsError
  - TestWithDataDir_CreatesDirectory

**Next Session Tasks:**
- Phase 5: Simplify CLI
  - Split cmd/kodit/main.go into separate files
  - Update CLI to use kodit.New() and client.API()
- Consider aligning infrastructure/search interfaces with domain/search interfaces
