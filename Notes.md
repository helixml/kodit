## Codebase Summary: Kodit

### Python Files & Lines of Code

| Section | Files | Lines of Code |
|---------|-------|---------------|
| **Source (src/)** | 234 | ~29,528 |
| **Tests (tests/)** | 109 | - |
| **Benchmarks (benchmarks/)** | 23,805 | ~294,000+ |
| **Scripts** | 1 | - |
| **Total** | ~45,901 | ~324,077 |

*Note: Benchmarks contain external test repositories (astropy, flask, django).*

---

### Main Directories

| Directory | Contents |
|-----------|----------|
| `src/kodit/` | Core application (218 files) - domain, infrastructure, application layers |
| `src/benchmark/` | Benchmarking utilities (16 files) |
| `tests/` | Pytest suite with unit, integration, regression, e2e tests |
| `benchmarks/repos/` | External project snapshots for testing |
| `docs/` | Documentation |
| `scripts/` | Utility scripts |
| `.github/` | GitHub Actions workflows |

---

### Entry Points

**CLI (`src/kodit/cli.py`):**
- `kodit serve` - Start FastAPI HTTP/SSE server (default: 127.0.0.1:8080)
- `kodit stdio` - Start MCP server in STDIO mode
- `kodit version` - Display version

**API (`src/kodit/app.py`):**
- `/api/v1/repositories` - Repository management
- `/api/v1/commits` - Commit operations
- `/api/v1/search` - Code search
- `/api/v1/enrichments` - Code enrichment
- `/api/v1/queue` - Task queue

---

### Testing Setup

- **Framework:** pytest with pytest-asyncio
- **Location:** `tests/` directory
- **Config:** `tests/conftest.py` + `pyproject.toml`
- **Coverage:** HTML, terminal, and XML reports via pytest-cov
- **Mode:** `asyncio_mode = "auto"`

---

### Key Dependencies

**Web/API:** FastAPI, uvicorn, httpx, fastmcp  
**Database:** SQLAlchemy (async), Alembic, aiosqlite, asyncpg  
**Code Analysis:** tree-sitter, tree-sitter-language-pack  
**Git:** gitpython, pygit2, dulwich  
**Embeddings/ML:** sentence-transformers, torch, transformers  
**LLM:** litellm, openai  
**Search:** bm25s  
**CLI:** click  
**Utilities:** pydantic, structlog, tiktoken  

**Python:** Requires >=3.12  
**Package Manager:** uv  
**Build:** hatchling + hatch-vcs

## Domain-Driven Design Bounded Contexts Analysis

The Kodit codebase has clear domain boundaries. Here's what I found:

---

### 1. **GIT MANAGEMENT CONTEXT**
**Purpose:** Manages Git repository operations, cloning, and commit scanning

**Paths:**
- `src/kodit/infrastructure/git/`
- `src/kodit/infrastructure/cloning/`
- `src/kodit/domain/services/git_repository_service.py`
- `src/kodit/domain/entities/git.py`

**Key Domain Objects:**
- `GitRepo` (Aggregate root)
- `GitCommit`, `GitBranch`, `GitTag`, `GitFile` (Entities)
- `RepositoryScanResult`, `TrackingConfig` (Value objects)

**Services:** `GitRepositoryScanner`, `RepositoryCloner`

**Repositories:** `GitRepoRepository`, `GitCommitRepository`, `GitFileRepository`, `GitBranchRepository`, `GitTagRepository`

**Communication:** Consumed by indexing services; exports aggregates; task operations: `CLONE_REPOSITORY`, `SYNC_REPOSITORY`, `SCAN_COMMIT`

---

### 2. **SNIPPET EXTRACTION & INDEXING CONTEXT**
**Purpose:** Extracts code snippets and creates searchable indexes

**Paths:**
- `src/kodit/infrastructure/slicing/`
- `src/kodit/infrastructure/indexing/`
- `src/kodit/infrastructure/bm25/`
- `src/kodit/infrastructure/embedding/`

**Key Domain Objects:**
- `SnippetV2` (Aggregate root - content-addressed by SHA)
- `CommitIndex` (Aggregate with status tracking)
- `Enrichment` (Value object)

**Services:** `BM25DomainService`, `EmbeddingDomainService`

**Repositories:** `SnippetRepositoryV2`, `BM25Repository`, `VectorSearchRepository`

**Communication:** Consumes Git context; consumed by Search and Enrichment contexts

---

### 3. **ENRICHMENT CONTEXT**
**Purpose:** Attaches semantic metadata through LLM-powered analysis

**Paths:**
- `src/kodit/domain/enrichments/` (main hierarchy)
  - `architecture/` - Physical architecture discovery
  - `development/` - Snippets, examples
  - `history/` - Commit descriptions
  - `usage/` - Cookbook, API docs
- `src/kodit/infrastructure/enricher/`
- `src/kodit/infrastructure/providers/litellm_provider.py`

**Key Domain Objects:**
- `EnrichmentV2` (Abstract base, frozen dataclass)
- Hierarchy: `ArchitectureEnrichment`, `DevelopmentEnrichment`, `HistoryEnrichment`, `UsageEnrichment`
- `EnrichmentAssociation`, `CommitEnrichmentAssociation`

**Services:** `PhysicalArchitectureService`, `CookbookContextService`

**Repositories:** `EnrichmentV2Repository`, `EnrichmentAssociationRepository`

---

### 4. **CODE SEARCH CONTEXT**
**Purpose:** Multi-modal search with hybrid retrieval (BM25 + vector)

**Paths:**
- `src/kodit/application/services/code_search_application_service.py`
- `src/kodit/infrastructure/api/v1/routers/search.py`
- `src/kodit/infrastructure/indexing/fusion_service.py`

**Key Domain Objects:**
- `MultiSearchRequest`, `MultiSearchResult`
- `SnippetSearchFilters` (language, author, dates, repo, enrichment types)
- `FusionRequest`, `FusionResult` (reciprocal rank fusion)

**Services:** `CodeSearchApplicationService`, `FusionService`

**Communication:** Consumes Snippet and Enrichment contexts; exposed via REST API

---

### 5. **TASK QUEUE & ORCHESTRATION CONTEXT**
**Purpose:** Async work queue and task execution workflow

**Paths:**
- `src/kodit/domain/entities/__init__.py` (Task, TaskStatus)
- `src/kodit/application/handlers/`
- `src/kodit/application/services/queue_service.py`
- `src/kodit/application/services/indexing_worker_service.py`

**Key Domain Objects:**
- `Task` (Entity with dedup_key, priority, payload)
- `TaskStatus` (Hierarchical progress tracking)
- `TaskOperation` (20+ operations enum)
- `QueuePriority`, `PrescribedOperations`

**Services:** `QueueService`, `CommitIndexingApplicationService`

**Communication:** Hub that orchestrates all other contexts through handlers

---

### 6. **REPOSITORY MANAGEMENT CONTEXT**
**Purpose:** High-level repository lifecycle operations

**Paths:**
- `src/kodit/application/handlers/repository/`
- `src/kodit/application/services/repository_query_service.py`
- `src/kodit/application/services/repository_sync_service.py`

**Key Domain Objects:**
- `Source` (Entity with WorkingCopy)
- `WorkingCopy` (Value object for cloned path/URI)

**Communication:** Consumes Git context; exposes via REST API `/api/v1/repositories`

---

### 7. **REPORTING & PROGRESS TRACKING CONTEXT**
**Purpose:** Real-time progress reporting for long-running tasks

**Paths:**
- `src/kodit/domain/tracking/`
- `src/kodit/application/services/reporting.py`
- `src/kodit/infrastructure/reporting/`

**Key Domain Objects:**
- `Trackable` (Interface), `TrackableType` (Enum)
- `RepositoryStatusSummary`, `IndexStatus`

**Services:** `TrackableResolutionService`, `TaskStatusQueryService`, `ProgressTracker`

---

### 8. **API GATEWAY CONTEXT**
**Purpose:** REST API interface

**Paths:**
- `src/kodit/infrastructure/api/v1/routers/`
- `src/kodit/infrastructure/api/v1/schemas/`

**Endpoints:** `/repositories`, `/commits`, `/search`, `/enrichments`, `/queue`

---

## Cross-Context Data Flow

```
Git Management
    │
    ├─→ Snippet Extraction
    │       │
    │       ├─→ Search Index (BM25 + embeddings)
    │       └─→ Enrichment Context
    │               │
    │               └─→ Code Search ──→ API Gateway
    │
    └─→ Repository Management ──→ Progress Tracking ──→ API Gateway
```

---

## Recommended Go Service Boundaries

1. **Git Service** - Cloning, scanning, branch/tag management
2. **Indexing Service** - Snippet extraction, BM25, embeddings  
3. **Enrichment Service** - LLM-powered analysis
4. **Search Service** - Multi-modal search, fusion
5. **Task Queue Service** - Async orchestration
6. **Repository API** - CRUD and lifecycle
7. **Data Layer** - Persistence (shared)
8. **Reporting Service** - Real-time progress

---

## Key Patterns to Preserve

- **Repository Pattern** - All data access through interfaces
- **Task Queue** - Dedup keys, priority-based execution
- **Value Objects** - Immutable (Enrichment, WorkingCopy, LanguageMapping)
- **Aggregate Roots** - GitRepo, SnippetV2, CommitIndex
- **Enrichment Hierarchy** - Polymorphic type/subtype structure
- **Hierarchical Task Status** - Parent/child progress tracking

Here's the domain vocabulary glossary extracted from the Kodit codebase:

## Domain Glossary - Kodit Code Understanding Platform

| Term | Definition | Where Used | Business Meaning |
|------|------------|------------|------------------|
| **GitRepo** | Core entity representing a tracked Git repository | `domain/entities/git.py` | A source code repository being analyzed for knowledge extraction |
| **WorkingCopy** | Local filesystem clone of a remote repository | `domain/entities/git.py`, `domain/value_objects.py` | The local checkout where code analysis happens |
| **TrackingConfig** | Configuration specifying which branch/tag/commit to monitor | `domain/entities/git.py` | Which version of the code to analyze and keep up-to-date |
| **RepositoryScanResult** | Immutable snapshot of branches, commits, files, tags | `domain/entities/git.py` | Complete metadata extracted from a repository scan |
| **SnippetV2** | Content-addressed code fragment (identified by SHA256) | `domain/entities/git.py` | A meaningful unit of code (function, class) extracted for analysis |
| **CommitIndex** | Container for all snippets from a single commit | `domain/entities/` | The indexed knowledge state of a specific commit |
| **Enrichment** | Semantic metadata attached to code via LLM analysis | `domain/enrichments/enrichment.py` | AI-generated insights about code purpose and structure |
| **PhysicalArchitecture** | System structure discovery (containers, services) | `infrastructure/physical_architecture/` | How the codebase is deployed/organized into components |
| **DatabaseSchema** | Extracted data model from migrations/models | Enrichment subtype | The data structures persisted by the application |
| **Cookbook** | Usage recipes showing how to accomplish tasks | Enrichment subtype | Step-by-step guides generated from real code patterns |
| **APIDoc** | Public interface documentation | `infrastructure/slicing/api_doc_extractor.py` | Documented entry points consumers can use |
| **Example** | Runnable code sample found in repository | `infrastructure/example_extraction/` | Executable demonstrations of functionality |
| **Slicer** | AST-based code extractor using tree-sitter | `infrastructure/slicing/slicer.py` | Intelligent parser that identifies semantic code units |
| **BM25 Index** | Keyword-based full-text search index | `domain/services/bm25_service.py` | Finding code by exact terms and phrases |
| **Vector Index** | Semantic similarity search via embeddings | `domain/services/embedding_service.py` | Finding conceptually similar code regardless of wording |
| **Fusion** | Combining BM25 + vector results via reciprocal rank | `domain/services/` | Hybrid search giving best of both ranking methods |
| **Task** | Unit of async work with deduplication | `domain/entities/`, `application/services/queue_service.py` | A queued operation ensuring work happens exactly once |
| **TaskStatus** | Hierarchical progress tracking | `domain/entities/` | Real-time visibility into long-running operations |
| **Trackable** | Reference point (branch/tag/commit) being processed | `domain/entities/` | The specific version being analyzed or indexed |
| **LanguageMapping** | Bidirectional language ↔ file extension mapping | `domain/value_objects.py` | Supported programming languages for analysis |
| **Author** | Commit author with name and email | `domain/entities/git.py` | Who made changes, for filtering and attribution |
| **DedupKey** | Unique identifier preventing duplicate work | Task entity | Idempotency key ensuring operations run once |

## Enrichment Type Hierarchy

| Type | Subtype | Business Meaning |
|------|---------|------------------|
| **architecture** | physical | How system components are deployed and connected |
| **architecture** | database_schema | Persistent data structures and relationships |
| **development** | snippet | Code patterns extracted for learning |
| **development** | snippet_summary | Aggregated pattern descriptions |
| **development** | example | Runnable code demonstrations |
| **development** | example_summary | Categorized example overview |
| **history** | commit_description | Why changes were made |
| **usage** | cookbook | Task-oriented usage guides |
| **usage** | api_docs | Public interface documentation |

## Task Operations (Domain Workflows)

| Operation | Business Meaning |
|-----------|------------------|
| CLONE_REPOSITORY | Download remote repo for analysis |
| SYNC_REPOSITORY | Update local copy with remote changes |
| SCAN_COMMIT | Extract metadata (files, commits, branches) |
| EXTRACT_SNIPPETS | Parse source into analyzable code units |
| CREATE_BM25_INDEX | Build keyword search capability |
| CREATE_CODE_EMBEDDINGS | Build semantic search capability |
| CREATE_*_ENRICHMENT | Generate AI-powered insights |

I've completed a thorough analysis of the Python codebase. Let me compile the findings into a comprehensive translation strategy document.

## Python-to-Go Translation Strategy

### 1. Decorators

| Decorator | Files | Go Equivalent |
|-----------|-------|---------------|
| `@dataclass(frozen=True)` | 35+ uses in `domain/`, `infrastructure/` | Struct with exported fields; immutability via unexported fields + constructor |
| `@property` | 36+ uses | Method with receiver (getter pattern) |
| `@abstractmethod` | 8+ uses | Interface definition |
| `@staticmethod` | 26+ uses | Package-level function or method without receiver |
| `@classmethod` | In `value_objects.py`, `config.py` | Constructor function or package-level function |
| `@field_validator` (Pydantic) | `config.py` | Custom `UnmarshalJSON` or validation in constructor |
| `@asynccontextmanager` | `mcp.py`, `app.py` | Resource cleanup via `defer` or explicit `Close()` methods |
| `@lru_cache` | `log.py` | `sync.Once` or manual caching with `sync.Map` |
| `@mcp_server.tool()` | `mcp.py` | Handler registration pattern |
| `@click.*` | CLI modules | `cobra` or `urfave/cli` decorators |
| `@router.*` (FastAPI) | API routers | `chi`, `gin`, or `echo` route registration |

**Go Example - Frozen Dataclass:**
```go
type Enrichment struct {
    typ     string  // unexported for immutability
    content string
}

func NewEnrichment(typ, content string) Enrichment {
    return Enrichment{typ: typ, content: content}
}

func (e Enrichment) Type() string    { return e.typ }
func (e Enrichment) Content() string { return e.content }
```

---

### 2. Abstract Base Classes

**15 ABCs found with 47+ implementations:**

| ABC | Location | Go Equivalent |
|-----|----------|---------------|
| `SqlAlchemyRepository[T,E]` | `infrastructure/sqlalchemy/repository.py` | Generic interface + embedding |
| `Query` | `infrastructure/sqlalchemy/query.py` | Interface with `Apply(stmt, model) Select` |
| `LanguageAnalyzer` | `infrastructure/slicing/language_analyzer.py` | Interface with 6 methods |
| `DocumentationParser` | `infrastructure/example_extraction/parser.py` | Interface with `Parse(string) []CodeBlock` |
| `EmbeddingProvider` | `domain/services/embedding_service.py` | Interface with channel return |
| `VectorSearchRepository` | `domain/services/embedding_service.py` | Interface |
| `BM25Repository` | `domain/services/bm25_service.py` | Interface |
| `EnrichmentV2` | `domain/enrichments/enrichment.py` | Interface with `Type()`, `Subtype()`, `EntityTypeKey()` |

**Go Example - Repository Interface:**
```go
type Repository[T any] interface {
    Get(ctx context.Context, id any) (T, error)
    Find(ctx context.Context, query Query) ([]T, error)
    Save(ctx context.Context, entity T) error
    Delete(ctx context.Context, entity T) error
}

type GitFileRepository interface {
    Repository[GitFile]
    DeleteByCommitSHA(ctx context.Context, sha string) error
}
```

---

### 3. Exception Handling

**Custom Exceptions Found:**

| Python Exception | Location | Go Pattern |
|------------------|----------|------------|
| `KoditAPIError` | `infrastructure/api/client/exceptions.py` | Sentinel error or custom type |
| `AuthenticationError` | Same | `errors.Is()` check |
| `ServerError` | Same | Custom error type with status |
| `EmptySourceError` | `domain/errors.py` | Sentinel: `var ErrEmptySource = errors.New(...)` |
| `EvaluationError` | `benchmark/` | Custom error type |
| `RepositoryCloneError` | `benchmark/` | `fmt.Errorf("clone failed: %w", err)` |

**Go Example - Error Hierarchy:**
```go
var (
    ErrKoditAPI        = errors.New("kodit API error")
    ErrAuthentication  = fmt.Errorf("%w: authentication failed", ErrKoditAPI)
    ErrConnection      = fmt.Errorf("%w: connection failed", ErrKoditAPI)
)

type ServerError struct {
    StatusCode int
    Message    string
}

func (e *ServerError) Error() string { return e.Message }
func (e *ServerError) Unwrap() error { return ErrKoditAPI }
```

**Exception Chaining → Error Wrapping:**
```go
// Python: raise ValueError("msg") from e
// Go:
return fmt.Errorf("invalid date format: %w", err)
```

---

### 4. Duck Typing

| Pattern | Location | Go Equivalent |
|---------|----------|---------------|
| `Protocol` classes | `domain/protocols.py` | Interface (structural typing) |
| `hasattr()` checks | 10+ locations | Type assertion or interface check |
| `getattr()` dynamic access | 15+ locations | Reflection (`reflect` package) or interface |
| `Any` parameters | Repositories | `any` type with type assertions |
| `isinstance()` checks | Query builders | Type switch |

**Go Example - Protocol to Interface:**
```go
// Python Protocol with optional method
type LanguageAnalyzer interface {
    NodeTypes() LanguageNodeTypes
    ExtractFunctionName(node Node) string
}

// Optional method via type assertion
if extractor, ok := analyzer.(TypeReferenceExtractor); ok {
    refs := extractor.ExtractTypeReferences(node)
}
```

---

### 5. Metaclasses / Dynamic Class Behavior

**None found** - the codebase doesn't use metaclasses, `__init_subclass__`, or `__new__` overrides. No translation needed.

---

### 6. Dependency Injection

| Pattern | Location | Go Equivalent |
|---------|----------|---------------|
| Constructor injection | Handlers, Services | Same - constructor functions |
| FastAPI `Depends()` | `dependencies.py` | Middleware or explicit wiring |
| Global singletons | `app.py` | `sync.Once` or wire at startup |
| Factory classes | `ServerFactory` | Factory struct or functions |

**Go Example - Constructor Injection:**
```go
type ScanCommitHandler struct {
    repoRepository      GitRepoRepository
    commitRepository    GitCommitRepository
    fileRepository      GitFileRepository
    scanner             GitRepositoryScanner
    operation           ProgressTracker
}

func NewScanCommitHandler(
    repoRepo GitRepoRepository,
    commitRepo GitCommitRepository,
    fileRepo GitFileRepository,
    scanner GitRepositoryScanner,
    op ProgressTracker,
) *ScanCommitHandler {
    return &ScanCommitHandler{
        repoRepository:   repoRepo,
        commitRepository: commitRepo,
        fileRepository:   fileRepo,
        scanner:          scanner,
        operation:        op,
    }
}
```

**Go Example - Factory:**
```go
type ServerFactory struct {
    appContext     *AppContext
    sessionFactory func() *sql.DB
    
    repoRepository *GitRepoRepository  // lazy-loaded
    mu             sync.Mutex
}

func (f *ServerFactory) RepoRepository() GitRepoRepository {
    f.mu.Lock()
    defer f.mu.Unlock()
    if f.repoRepository == nil {
        f.repoRepository = NewGitRepoRepository(f.sessionFactory)
    }
    return f.repoRepository
}
```

---

### 7. Configuration

| Pattern | Location | Go Equivalent |
|---------|----------|---------------|
| Pydantic Settings | `config.py` | `envconfig`, `viper`, or `koanf` |
| `.env` file loading | `AppContext` | `godotenv` |
| Nested config models | `Endpoint`, `Search`, etc. | Nested structs with tags |
| `@field_validator` | API key parsing | Custom `UnmarshalText` |
| Default values | Field defaults | Struct tags or explicit defaults |

**Go Example - Configuration:**
```go
type AppContext struct {
    DataDir          string         `env:"DATA_DIR" default:"~/.kodit"`
    DBUrl            string         `env:"DB_URL"`
    LogLevel         string         `env:"LOG_LEVEL" default:"INFO"`
    DisableTelemetry bool           `env:"DISABLE_TELEMETRY" default:"false"`
    APIKeys          []string       `env:"API_KEYS"` // needs custom unmarshaler
    Embedding        EndpointConfig `envPrefix:"EMBEDDING_ENDPOINT_"`
    Remote           RemoteConfig   `envPrefix:"REMOTE_"`
}

func (c *AppContext) UnmarshalAPIKeys(value string) error {
    c.APIKeys = strings.Split(value, ",")
    return nil
}
```

---

### 8. Async Code

| Pattern | Location | Go Equivalent |
|---------|----------|---------------|
| `async def` | Throughout | Regular function with `context.Context` |
| `await` | Database, HTTP calls | Blocking call (Go handles concurrency) |
| `asyncio.Task` | Workers, schedulers | Goroutine |
| `asyncio.Event` | Shutdown signals | `context.Context` cancellation |
| `async with` (context mgr) | Unit of work | `defer` or explicit cleanup |
| `AsyncIterator`/`yield` | Streaming results | Channel (`chan T`) |

**Go Example - Async Worker:**
```go
type IndexingWorker struct {
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

func (w *IndexingWorker) Start() {
    w.wg.Add(1)
    go func() {
        defer w.wg.Done()
        w.workerLoop()
    }()
}

func (w *IndexingWorker) Stop() {
    w.cancel()
    w.wg.Wait()
}

func (w *IndexingWorker) workerLoop() {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-w.ctx.Done():
            return
        case <-ticker.C:
            if task, err := w.dequeueTask(); err == nil {
                w.processTask(task)
            }
        }
    }
}
```

**Go Example - AsyncIterator to Channel:**
```go
// Python: async def embed(...) -> AsyncGenerator[list[EmbeddingResponse], None]
// Go:
func (p *EmbeddingProvider) Embed(ctx context.Context, requests []EmbeddingRequest) <-chan []EmbeddingResponse {
    results := make(chan []EmbeddingResponse)
    go func() {
        defer close(results)
        for batch := range batches(requests) {
            select {
            case <-ctx.Done():
                return
            case results <- p.processBatch(batch):
            }
        }
    }()
    return results
}
```

---

### Summary of Key Translations

| Python Concept | Go Equivalent | Notes |
|----------------|---------------|-------|
| `@dataclass(frozen=True)` | Struct + private fields + getters | Immutability by convention |
| `ABC` + `@abstractmethod` | Interface | Natural fit |
| `Protocol` | Interface | Structural typing matches |
| Exception hierarchy | Error wrapping + `errors.Is/As` | Use sentinel errors |
| `async/await` | Goroutines + channels + context | Blocking calls are fine |
| Pydantic Settings | `envconfig`/`viper` + struct tags | Similar pattern |
| Factory pattern | Factory functions/structs | Same |
| DI | Constructor injection | Same, or use `wire` |

Here's the comprehensive dependency map:

## 1. Database/ORM

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| **SQLAlchemy** 2.0.40+ | Async ORM with PostgreSQL/SQLite support | `database.py`, `infrastructure/sqlalchemy/entities.py`, `infrastructure/sqlalchemy/repository.py`, all migrations | **sqlc** (type-safe SQL) or **GORM** (ORM) or **sqlx** (raw SQL) |
| **Alembic** 1.15.2+ | Database migrations | `database.py`, `migrations/env.py` | **golang-migrate/migrate** or **goose** |
| **aiosqlite** 0.20.0+ | Async SQLite driver | Via SQLAlchemy | **modernc.org/sqlite** (pure Go) |
| **asyncpg** 0.30.0+ | Async PostgreSQL driver | Via SQLAlchemy | **pgx** or **lib/pq** |

**Key Features Used:** `AsyncSession`, `async_sessionmaker`, `DeclarativeBase`, `Mapped`, `mapped_column`, custom `TypeDecorator` (TZDateTime, PathType), event listeners, generic repository pattern

---

## 2. Web Framework

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| **FastAPI** 0.115.12+ | Async REST API framework | `app.py`, all `infrastructure/api/v1/routers/` | **Chi**, **Echo**, or **Gin** |
| **Uvicorn** (via FastAPI) | ASGI server | `cli.py` | Built-in `net/http` or **Chi** |
| **asgi-correlation-id** 4.3.4+ | Request correlation tracking | `app.py`, `middleware.py` | Custom middleware with `context.Context` |
| **Pydantic** 2.9.1+ | Request/response validation | `infrastructure/api/v1/schemas/`, `domain/entities/`, `config.py` | **go-playground/validator** + struct tags |
| **pydantic-settings** 2.9.1+ | Environment config | `config.py` | **kelseyhightower/envconfig** or **spf13/viper** |

**Key Features Used:** `APIRouter`, `Depends()` (DI), `HTTPException`, `Query`, `Security`, `APIKeyHeader`, lifespan context managers, MCP server mounting

---

## 3. Message Queue / Events

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| *None identified* | — | — | **NATS**, **RabbitMQ/amqp**, **watermill** |

---

## 4. External APIs / HTTP Clients

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| **httpx** 0.28.1+ | Async HTTP client | `infrastructure/providers/litellm_provider.py`, `infrastructure/api/client/base.py` | **net/http** (stdlib) or **resty** |
| **httpx-retries** 0.3.2+ | Retry middleware | Via httpx | **hashicorp/go-retryablehttp** |
| **litellm** 1.81.5+ | Unified LLM provider (100+ providers) | `infrastructure/providers/litellm_provider.py`, `infrastructure/embedding/`, `infrastructure/enricher/` | **sashabaranov/go-openai** + custom provider abstraction |
| **openai** 2.8.0+ | Direct OpenAI client | Via litellm | **sashabaranov/go-openai** |

**LiteLLM Features Used:** `acompletion()`, `aembedding()`, disk caching, retry with exponential backoff, multiple exception types

---

## 5. Serialization / Validation

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| **Pydantic** 2.9.1+ | Data validation via type hints | `domain/entities/`, `domain/value_objects.py`, `infrastructure/api/v1/schemas/` | **go-playground/validator** |
| **Jinja2** 3.1.0+ | Template rendering | `infrastructure/slicing/formatters/template_formatter.py`, `utils/dump_config.py` | **text/template** or **html/template** (stdlib) |

**Pydantic Features Used:** `BaseModel`, `Field()`, `field_validator()`, `AnyUrl`, model serialization

---

## 6. Testing

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| **pytest** 8.3.5+ | Test framework | All tests | **testing** (stdlib) + **testify** |
| **pytest-asyncio** 0.26.0+ | Async test support | Async tests | N/A (Go handles concurrency natively) |
| **pytest-cov** 6.1.1+ | Coverage reporting | Test runs | `go test -cover` |
| **mypy** 1.15.0+ | Static type checking | CI | N/A (Go is statically typed) |
| **ruff** 0.11.8+ | Linter/formatter | CI | **golangci-lint** |
| **swebench** 4.1.0+ | Code generation benchmarks | `benchmark/swebench/` | Custom implementation |

---

## 7. Utilities

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| **Click** 8.1.8+ | CLI framework | `cli.py`, `config.py` | **spf13/cobra** or **urfave/cli** |
| **structlog** 25.3.0+ | Structured logging | All modules via `log.py` | **rs/zerolog** or **uber-go/zap** |
| **aiofiles** 24.1.0+ | Async file I/O | `infrastructure/bm25/local_bm25_repository.py` | **os** (stdlib) with goroutines |
| **pathspec** 0.12.1+ | Gitignore pattern matching | `infrastructure/ignore/ignore_pattern_provider.py` | **go-git/go-git** or **bmatcuk/doublestar** |
| **rudder-sdk-python** 2.1.4+ | Telemetry/analytics | `log.py` | **rudderlabs/analytics-go** |
| **better-exceptions** 0.3.3+ | Readable tracebacks | CLI | **pkg/errors** or **cockroachdb/errors** |

---

## Domain-Critical: Code Analysis & Search

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| **tree-sitter** 0.24.0+ | Multi-language AST parsing | `infrastructure/slicing/slicer.py`, `ast_analyzer.py`, `language_analyzer.py` | **smacker/go-tree-sitter** |
| **tree-sitter-language-pack** 0.7.3+ | Pre-compiled grammars | `infrastructure/slicing/language_config.py` | Individual grammars per language |
| **bm25s** 0.2.12+ | BM25 full-text search | `infrastructure/bm25/local_bm25_repository.py`, `vectorchord_bm25_repository.py` | **blevesearch/bleve** or custom BM25 |
| **pystemmer** 3.0.0+ | Text stemming | `infrastructure/bm25/local_bm25_repository.py` | **kljensen/snowball** |
| **tiktoken** 0.9.0+ | Token counting | Multiple embedding providers, `local_enricher.py` | **pkoukk/tiktoken-go** |
| **sentence-transformers** 4.1.0+ | Local embeddings | `infrastructure/embedding/embedding_providers/local_embedding_provider.py` | Call via **ONNX Runtime** or external API |

---

## Domain-Critical: Git Operations

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| **GitPython** 3.1.44+ | Git repo interface | `infrastructure/cloning/git/git_python_adaptor.py`, `ignore_pattern_provider.py` | **go-git/go-git** |
| **pygit2** 1.19.1 | libgit2 bindings | `infrastructure/cloning/git/pygit2_adaptor.py` | **libgit2/git2go** (CGo) |
| **dulwich** 0.22.0+ | Pure Python Git | `infrastructure/cloning/git/dulwich_adaptor.py` | **go-git/go-git** (pure Go) |

**Git Features Used:** Commit iteration, tree/blob inspection, branch/tag listing, diff analysis, gitignore matching

---

## MCP (Model Context Protocol)

| Python Library | Purpose | Files Using It | Suggested Go Equivalent |
|----------------|---------|----------------|------------------------|
| **fastmcp** 2.10.4+ | MCP server framework | `mcp.py`, `app.py` | **mark3labs/mcp-go** |

---

## Summary: Critical Migration Considerations

1. **Async → Goroutines**: Replace `async/await` patterns with goroutines and channels
2. **SQLAlchemy → sqlc/GORM**: ORM patterns differ significantly; sqlc provides type-safe SQL generation
3. **Pydantic → Struct Tags**: Validation moves to struct tags + validator library
4. **LiteLLM abstraction**: No direct equivalent; build provider interface with `go-openai` as base
5. **tree-sitter bindings**: `go-tree-sitter` exists but may need CGo; evaluate performance
6. **Local ML (sentence-transformers/torch)**: No pure-Go equivalent; use ONNX Runtime or external service

## Repository Pattern Analysis

Here's a comprehensive breakdown of how the repository pattern is implemented:

### 1. Repository Interfaces (Protocols)

Located in `src/kodit/domain/protocols.py`:

```python
class Repository[T](Protocol):
    async def get(self, entity_id: Any) -> T
    async def get_or_create(self, entity: T, unique_field: str) -> tuple[T, bool]
    async def find(self, query: Query) -> list[T]
    async def save(self, entity: T) -> T
    async def save_bulk(self, entities: list[T], *, skip_existence_check: bool = False) -> list[T]
    async def exists(self, entity_id: Any) -> bool
    async def delete(self, entity: T) -> None
    async def delete_by_query(self, query: Query) -> None
    async def count(self, query: Query) -> int
```

**Specialized interfaces** (same file):
- `GitRepoRepository`, `GitCommitRepository`, `GitFileRepository`, `GitBranchRepository`, `GitTagRepository`
- `TaskRepository`, `TaskStatusRepository`
- `EnrichmentV2Repository`, `EnrichmentAssociationRepository`

### 2. Query Construction - Query Builder Pattern

Located in `src/kodit/infrastructure/sqlalchemy/query.py`:

```python
class QueryBuilder(Query):
    def filter(self, field: str, operator: FilterOperator, value: Any) -> Self
    def sort(self, field: str, *, descending: bool = False) -> Self
    def paginate(self, pagination: PaginationParams) -> Self
    def apply(self, stmt: Select, model_type: type) -> Select
```

**FilterOperator enum**: `EQ`, `NE`, `GT`, `GTE`, `LT`, `LTE`, `IN`, `LIKE`, `ILIKE`

**Usage example**:
```python
query = (
    QueryBuilder()
    .filter("repo_id", FilterOperator.EQ, repo_id)
    .filter("name", FilterOperator.EQ, branch_name)
    .sort("created_at", descending=True)
)
results = await repository.find(query)
```

### 3. Base Repository Implementation

Located in `src/kodit/infrastructure/sqlalchemy/repository.py`:

```python
class SqlAlchemyRepository(ABC, Generic[DomainEntityType, DatabaseEntityType]):
    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None
    
    # Abstract (subclasses must implement):
    def _get_id(self, entity: DomainEntityType) -> Any
    def db_entity_type(self) -> type[DatabaseEntityType]
    def to_domain(db_entity: DatabaseEntityType) -> DomainEntityType
    def to_db(domain_entity: DomainEntityType) -> DatabaseEntityType
```

### 4. Example Concrete Repository

From `src/kodit/infrastructure/sqlalchemy/git_branch_repository.py`:

```python
class SqlAlchemyGitBranchRepository(
    SqlAlchemyRepository[GitBranch, db_entities.GitBranch], 
    GitBranchRepository
):
    def _get_id(self, entity: GitBranch) -> tuple[int, str]:
        return (entity.repo_id, entity.name)  # Composite key

    @property
    def db_entity_type(self) -> type[db_entities.GitBranch]:
        return db_entities.GitBranch

    @staticmethod
    def to_domain(db_entity: db_entities.GitBranch) -> GitBranch:
        return GitBranch(
            repo_id=db_entity.repo_id,
            name=db_entity.name,
            head_commit_sha=db_entity.head_commit_sha,
        )

    @staticmethod
    def to_db(domain_entity: GitBranch) -> db_entities.GitBranch:
        return db_entities.GitBranch(
            repo_id=domain_entity.repo_id,
            name=domain_entity.name,
            head_commit_sha=domain_entity.head_commit_sha,
        )

    # Specialized method
    async def get_by_name(self, branch_name: str, repo_id: int) -> GitBranch:
        query = QueryBuilder().filter("name", FilterOperator.EQ, branch_name)
        branches = await self.find(query)
        return branches[0]
```

### 5. Transaction Handling - Unit of Work

Located in `src/kodit/infrastructure/sqlalchemy/unit_of_work.py`:

```python
class SqlAlchemyUnitOfWork:
    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None
    async def __aenter__(self) -> AsyncSession
    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None  # Auto commit/rollback
    async def commit(self) -> None
    async def rollback(self) -> None
    async def flush(self) -> None
```

**Pattern**: Each repository method wraps operations in UoW:
```python
async def save(self, entity: DomainEntityType) -> DomainEntityType:
    async with SqlAlchemyUnitOfWork(self.session_factory) as session:
        # ... do work
        await session.flush()
        return self.to_domain(db_entity)
    # Auto-commits on success, rolls back on exception
```

### 6. Dependency Injection - Factory Pattern

Located in `src/kodit/application/factories/server_factory.py`:

```python
class ServerFactory:
    def __init__(self, app_context: AppContext, session_factory: Callable[[], AsyncSession]):
        self.session_factory = session_factory
    
    def git_repo_repository(self) -> GitRepoRepository:
        if not self._git_repo_repository:
            self._git_repo_repository = SqlAlchemyGitRepoRepository(
                session_factory=self.session_factory
            )
        return self._git_repo_repository
```

Each repository module also exports a factory function:
```python
def create_git_branch_repository(session_factory: Callable[[], AsyncSession]) -> GitBranchRepository:
    return SqlAlchemyGitBranchRepository(session_factory=session_factory)
```

---

### Go Translation Guidance

| Python | Go Equivalent |
|--------|---------------|
| `Protocol` | Interface |
| `Generic[T]` | Generics with type constraints |
| `async/await` | Goroutines or standard functions |
| `SqlAlchemyUnitOfWork` | `*sql.Tx` with defer/recover |
| `session_factory` | `*sql.DB` or connection pool |
| `QueryBuilder` | Squirrel, sqlc, or custom builder |
| `ServerFactory` | Wire, dig, or manual DI |
| `@staticmethod to_domain/to_db` | Mapper functions or methods |

## Event-Driven Patterns Found

The codebase uses a **database-backed task queue** pattern (no external message broker like Kafka/RabbitMQ).

---

### 1. Event Types (TaskOperation enum)

**Location:** `src/kodit/domain/value_objects.py:605-658`

30+ operation types including:
- Repository: `CLONE_REPOSITORY`, `SYNC_REPOSITORY`, `DELETE_REPOSITORY`
- Commit processing: `SCAN_COMMIT`, `EXTRACT_SNIPPETS_FOR_COMMIT`, `CREATE_CODE_EMBEDDINGS_FOR_COMMIT`, etc.

---

### 2. Event Payload (Task entity)

**Location:** `src/kodit/domain/entities/__init__.py:151-183`

```python
Task(
    dedup_key: str,           # Idempotent key
    type: TaskOperation,      # Event type
    priority: int,
    payload: dict[str, Any],  # Event data
)
```

---

### 3. Event Publisher (QueueService)

**Location:** `src/kodit/application/services/queue_service.py`

- `enqueue_task(task)` - Publish single event
- `enqueue_tasks(tasks, priority, payload)` - Publish batch with choreography

Uses upsert semantics for idempotent publishing.

---

### 4. Event Handlers

**Location:** `src/kodit/application/handlers/`

| Handler | Event Type | Async |
|---------|-----------|-------|
| `CloneRepositoryHandler` | CLONE_REPOSITORY | ✓ |
| `SyncRepositoryHandler` | SYNC_REPOSITORY | ✓ |
| `ScanCommitHandler` | SCAN_COMMIT | ✓ |
| `ExtractSnippetsHandler` | EXTRACT_SNIPPETS_FOR_COMMIT | ✓ |
| `CreateBM25IndexHandler` | CREATE_BM25_INDEX_FOR_COMMIT | ✓ |
| (21 total handlers) | ... | ✓ |

All implement: `async def execute(payload: dict[str, Any]) -> None`

---

### 5. Handler Registry (Dispatcher)

**Location:** `src/kodit/application/handlers/registry.py`

```python
class TaskHandlerRegistry:
    register(operation: TaskOperation, handler: TaskHandler)
    handler(operation: TaskOperation) -> TaskHandler
```

Registration at: `src/kodit/application/factories/server_factory.py:312-450`

---

### 6. Event Consumer (Worker)

**Location:** `src/kodit/application/services/indexing_worker_service.py`

- Polls database for pending tasks
- Processes one at a time
- Deletes task after success

---

### 7. Progress/Notification System (Observer Pattern)

**Location:** `src/kodit/application/services/reporting.py`

**Publisher:** `ProgressTracker` with `notify_subscribers()`

**Subscribers:** (via `ReportingModule` protocol)
- `LoggingReportingModule` - Logs progress
- `DBProgressReportingModule` - Persists to DB
- `TelemetryProgressReportingModule` - Rudderstack analytics

**Progress Event:** `TaskStatus` entity with state machine (STARTED → IN_PROGRESS → COMPLETED/FAILED/SKIPPED)

---

### 8. Choreography (Workflow Pipelines)

**Location:** `src/kodit/domain/value_objects.py:660-697`

`PrescribedOperations` defines sequences:
- `SCAN_AND_INDEX_COMMIT` → [SCAN → EXTRACT_SNIPPETS → EXTRACT_EXAMPLES → CREATE_BM25_INDEX → CREATE_EMBEDDINGS → ...]

---

### Event Flow

```
Publisher (QueueService)
    ↓ enqueue_tasks()
Database (Task Queue)
    ↓ polling
IndexingWorkerService
    ↓ handler lookup
TaskHandlerRegistry → Handler.execute()
    ↓ progress updates
ProgressTracker.notify_subscribers()
    ↓
[Logging, DB, Telemetry] subscribers
```

**Key traits:** All async, database-backed queue, idempotent publishing, choreography-based workflows.