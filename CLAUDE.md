# Kodit Python-to-Go Migration Guide

## Project Overview

Kodit is a code understanding platform that indexes Git repositories, extracts semantic code snippets using AST parsing, and provides hybrid search (BM25 + vector embeddings) with LLM-powered enrichments. The system processes repositories through a task queue, building searchable indexes and generating documentation, API docs, and usage examples.

---

## Bounded Contexts

### 1. Git Management

**Purpose:** Manages Git repository operations, cloning, and commit scanning

| Attribute | Value |
|-----------|-------|
| Python source | `src/kodit/infrastructure/git/`, `src/kodit/infrastructure/cloning/`, `src/kodit/domain/services/git_repository_service.py`, `src/kodit/domain/entities/git.py` |
| Go target | `internal/git/` |
| Aggregates | `GitRepo` (root) |
| Entities | `GitCommit`, `GitBranch`, `GitTag`, `GitFile` |
| Value Objects | `RepositoryScanResult`, `TrackingConfig`, `WorkingCopy`, `Author` |
| Repositories | `GitRepoRepository`, `GitCommitRepository`, `GitFileRepository`, `GitBranchRepository`, `GitTagRepository` |
| Events | Publishes: `CLONE_REPOSITORY`, `SYNC_REPOSITORY`, `SCAN_COMMIT` |
| Dependencies | None (root context) |

### 2. Snippet Extraction & Indexing

**Purpose:** Extracts code snippets via AST parsing and creates searchable indexes

| Attribute | Value |
|-----------|-------|
| Python source | `src/kodit/infrastructure/slicing/`, `src/kodit/infrastructure/indexing/`, `src/kodit/infrastructure/bm25/`, `src/kodit/infrastructure/embedding/` |
| Go target | `internal/indexing/` |
| Aggregates | `SnippetV2` (content-addressed by SHA256), `CommitIndex` |
| Value Objects | `LanguageMapping`, `SnippetSearchFilters` |
| Repositories | `SnippetRepositoryV2`, `BM25Repository`, `VectorSearchRepository` |
| Events | Consumes: `SCAN_COMMIT`; Publishes: `EXTRACT_SNIPPETS_FOR_COMMIT`, `CREATE_BM25_INDEX_FOR_COMMIT`, `CREATE_CODE_EMBEDDINGS_FOR_COMMIT` |
| Dependencies | Git Management, AI Provider (`internal/provider/` for embeddings) |

### 3. Enrichment

**Purpose:** Attaches semantic metadata through LLM-powered analysis

| Attribute | Value |
|-----------|-------|
| Python source | `src/kodit/domain/enrichments/`, `src/kodit/infrastructure/enricher/`, `src/kodit/infrastructure/providers/litellm_provider.py` |
| Go target | `internal/enrichment/` |
| Aggregates | `EnrichmentV2` (abstract base with hierarchy) |
| Subtypes | `ArchitectureEnrichment` (physical, database_schema), `DevelopmentEnrichment` (snippet, example), `HistoryEnrichment` (commit_description), `UsageEnrichment` (cookbook, api_docs) |
| Value Objects | `EnrichmentAssociation`, `CommitEnrichmentAssociation` |
| Repositories | `EnrichmentV2Repository`, `EnrichmentAssociationRepository` |
| Events | Consumes: `EXTRACT_SNIPPETS_FOR_COMMIT`; Publishes: `CREATE_*_ENRICHMENT` operations |
| Dependencies | Snippet Extraction, AI Provider (`internal/provider/` for text generation) |

### 4. Code Search

**Purpose:** Multi-modal search with hybrid retrieval (BM25 + vector)

| Attribute | Value |
|-----------|-------|
| Python source | `src/kodit/application/services/code_search_application_service.py`, `src/kodit/infrastructure/api/v1/routers/search.py`, `src/kodit/infrastructure/indexing/fusion_service.py` |
| Go target | `internal/search/` |
| Value Objects | `MultiSearchRequest`, `MultiSearchResult`, `FusionRequest`, `FusionResult`, `SnippetSearchFilters` |
| Services | `CodeSearchApplicationService`, `FusionService` |
| Events | None (query-only) |
| Dependencies | Snippet Extraction, Enrichment |

### 5. Task Queue & Orchestration

**Purpose:** Async work queue and task execution workflow

| Attribute | Value |
|-----------|-------|
| Python source | `src/kodit/domain/entities/__init__.py`, `src/kodit/application/handlers/`, `src/kodit/application/services/queue_service.py`, `src/kodit/application/services/indexing_worker_service.py` |
| Go target | `internal/queue/` |
| Entities | `Task` (with dedup_key, priority, payload) |
| Value Objects | `TaskStatus`, `TaskOperation` (30+ operations), `QueuePriority`, `PrescribedOperations` |
| Services | `QueueService`, `IndexingWorkerService`, `TaskHandlerRegistry` |
| Events | Hub - consumes and publishes all task operations |
| Dependencies | All other contexts |

### 6. Repository Management

**Purpose:** High-level repository lifecycle operations

| Attribute | Value |
|-----------|-------|
| Python source | `src/kodit/application/handlers/repository/`, `src/kodit/application/services/repository_query_service.py`, `src/kodit/application/services/repository_sync_service.py` |
| Go target | `internal/repository/` |
| Entities | `Source` (with WorkingCopy) |
| Services | `RepositoryQueryService`, `RepositorySyncService` |
| Events | Triggers `CLONE_REPOSITORY`, `SYNC_REPOSITORY`, `DELETE_REPOSITORY` |
| Dependencies | Git Management, Task Queue |

### 7. Progress Tracking

**Purpose:** Real-time progress reporting for long-running tasks

| Attribute | Value |
|-----------|-------|
| Python source | `src/kodit/domain/tracking/`, `src/kodit/application/services/reporting.py`, `src/kodit/infrastructure/reporting/` |
| Go target | `internal/tracking/` |
| Interfaces | `Trackable`, `ReportingModule` |
| Value Objects | `TrackableType`, `RepositoryStatusSummary`, `IndexStatus` |
| Services | `ProgressTracker`, `TaskStatusQueryService`, `TrackableResolutionService` |
| Events | Subscribes to all task progress updates |
| Dependencies | Task Queue |

### 8. API Gateway

**Purpose:** REST API interface

| Attribute | Value |
|-----------|-------|
| Python source | `src/kodit/infrastructure/api/v1/routers/`, `src/kodit/infrastructure/api/v1/schemas/` |
| Go target | `internal/api/` |
| Endpoints | `/api/v1/repositories`, `/api/v1/commits`, `/api/v1/search`, `/api/v1/enrichments`, `/api/v1/queue` |
| Dependencies | All query services |

---

## Ubiquitous Language Glossary

| Term | Definition |
|------|------------|
| **GitRepo** | A tracked Git repository being analyzed for knowledge extraction |
| **WorkingCopy** | Local filesystem clone where code analysis happens |
| **TrackingConfig** | Branch/tag/commit to monitor and keep indexed |
| **SnippetV2** | Content-addressed code fragment (function, class) identified by SHA256 |
| **CommitIndex** | All snippets from a single commit - the indexed knowledge state |
| **Enrichment** | AI-generated semantic metadata about code purpose and structure |
| **PhysicalArchitecture** | System structure discovery (containers, services, deployment) |
| **Cookbook** | Task-oriented usage guides generated from code patterns |
| **APIDoc** | Public interface documentation extracted from code |
| **Slicer** | AST-based code extractor using tree-sitter |
| **BM25 Index** | Keyword-based full-text search index |
| **Vector Index** | Semantic similarity search via embeddings |
| **Fusion** | Combining BM25 + vector results via reciprocal rank fusion |
| **Task** | Unit of async work with deduplication key |
| **TaskStatus** | Hierarchical progress tracking with parent/child relationships |
| **DedupKey** | Idempotency key ensuring operations run exactly once |
| **Trackable** | Reference point (branch/tag/commit) being processed |

---

## Go Project Structure

This is the EXACT structure to use. Do not deviate.

```
[project-root]/
├── cmd/                          # Application entry points
│   └── kodit/
│       └── main.go
├── internal/                     # Private application code
│   ├── shared/                   # Cross-cutting concerns
│   │   ├── types/               # Common value objects (Money, ID types, etc.)
│   │   │   └── types.go
│   │   ├── errors/              # Shared error definitions
│   │   │   └── errors.go
│   │   └── events/              # Domain event interfaces and base types
│   │       └── event.go
│   │
│   ├── provider/                 # Unified AI provider abstraction
│   │   ├── provider.go          # Interface for text generation + embeddings
│   │   ├── openai.go            # OpenAI implementation
│   │   └── [other].go           # Additional providers as needed
│   │
│   └── [boundedcontext]/        # One directory per bounded context
│       ├── domain/              # Domain layer (innermost)
│       │   ├── [aggregate].go       # Aggregate root and entities
│       │   ├── repository.go        # Repository INTERFACE
│       │   ├── service.go           # Domain services
│       │   └── events.go            # Domain events for this context
│       │
│       ├── application/         # Application layer
│       │   ├── [usecase].go         # Use case / application service
│       │   ├── [usecase]_test.go
│       │   └── dto.go               # Data transfer objects (if needed)
│       │
│       └── infrastructure/      # Infrastructure layer (outermost)
│           ├── persistence/
│           │   └── postgres_[repo].go  # Repository implementation
│           └── adapters/
│               └── [external].go    # External service adapters
│
├── pkg/                         # Public libraries (if any)
├── migrations/                  # Database migrations
├── config/                      # Configuration files
├── go.mod
└── go.sum
```

### Layer Dependencies

- **Domain layer**: Pure business logic and domain model. No external dependencies except stdlib.
- **Application layer**: Can import domain. No infrastructure imports. Defines interfaces that infrastructure implements.
- **Infrastructure layer**: Can import domain and application. Implements interfaces defined elsewhere.

### File Naming

- One primary type per file (e.g., `order.go` contains `Order` struct)
- Test files: `[name]_test.go` in same package
- Repository interface: `repository.go` in domain
- Repository implementation: `postgres_repository.go` or `[db]_repository.go` in infrastructure

### Package Naming

- `domain` not `domains`
- `application` not `app` or `service`
- `infrastructure` not `infra`
- Context name should be singular: `order` not `orders`

---

## Coding Standards

### Object-Oriented Principles

- **Class Naming**: Name according to what it is, not what it does. Don't end names with -er. (1.1)
- **Method Naming**: Methods are either a builder or a manipulator. Never both. Use a noun if it's a builder, or a verb if it's a manipulator. (2.4)
- **Variable Naming**: If you can't explain the code using single and plural nouns, refactor. Avoid compound names where possible. (5.1)
- **Constructors**: Make one constructor primary, others must use this constructor. (1.2) Keep constructors free of code. (1.3) Don't use new anywhere except secondary constructors. (3.6)
- **Methods**: Expose fewer than five public methods. (3.1) Don't use static methods. (3.2) Never accept null arguments, encapsulate. (3.3) Don't use getters and setters. (3.5) Never return null. (4.1)
- **Encapsulation**: Classes should encapsulate four objects or less. (2.1) Use composition, not inheritance. (5.7)
- **Decoupling**: Use interfaces where possible. (2.3) Keep interfaces small. (2.9)
- **Globals**: Don't use public constants or enums, use classes instead. (2.5)
- **Immutability**: Make all classes immutable. (2.6) Avoid type introspection and reflection. (3.7, 6.4)
- **Testing**: Don't mock, use fakes. (2.8)
- **Design**: Think in objects, not algorithms. (5.10) Design methods by telling them what you want, don't ask for data. (5.3)

### Go Principles

- Write idiomatic Go, not "Python in Go syntax"
- ONLY exported functions/types need doc comments starting with name
- Explain why, not what (code shows what)
- Accept interfaces, return concrete types
- Make zero values useful
- Don't panic in library code

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Package | lowercase, single word | `git`, `indexing`, `queue` |
| Interface | noun or -er suffix | `Repository`, `Scanner`, `Handler` |
| Struct | noun | `GitRepo`, `Task`, `WorkingCopy` |
| Constructor | `New` prefix | `NewGitRepo()`, `NewQueueService()` |
| Getter | field name (no Get prefix) | `repo.ID()`, `task.Status()` |
| Errors | `Err` prefix | `ErrNotFound`, `ErrConflict` |
| Context | first param, named `ctx` | `func (s *Service) Do(ctx context.Context)` |

### Testing

- Table-driven tests for multiple cases
- Test file in same package: `foo_test.go`
- Use `testify/assert` or standard library
- Name tests: `TestFunctionName_Scenario`
- Don't mock, use fakes (test doubles that implement real interfaces)

---

## Python to Go Translation Rules

### Classes and Objects

| Python | Go | Notes |
|--------|-----|-------|
| `class Foo:` | `type Foo struct {}` | |
| `class Foo(Base):` | Embed `Base` or implement interface | Prefer composition |
| `@dataclass` | `type Foo struct {}` + `NewFoo()` constructor | |
| `@dataclass(frozen=True)` | Struct with unexported fields + getters | Value semantics, no pointer receivers |
| `__init__` | `NewFoo() (*Foo, error)` | Return error for validation |
| `__str__` | `func (f Foo) String() string` | Implement `fmt.Stringer` |
| `__eq__` | `func (f Foo) Equal(other Foo) bool` | Manual implementation |
| `@property` | Getter method `func (f *Foo) Name() string` | |
| `@name.setter` | Setter method `func (f *Foo) SetName(n string)` | Avoid if possible |
| `@staticmethod` | Package-level function | |
| `@classmethod` | Constructor variant `NewFooFromX()` | |
| `@abstractmethod` | Interface method | |

**Immutable Value Object Example:**
```go
type WorkingCopy struct {
    path string
    uri  string
}

func NewWorkingCopy(path, uri string) WorkingCopy {
    return WorkingCopy{path: path, uri: uri}
}

func (w WorkingCopy) Path() string { return w.path }
func (w WorkingCopy) URI() string  { return w.uri }
```

### Error Handling

| Python | Go | Notes |
|--------|-----|-------|
| `raise ValueError("msg")` | `return fmt.Errorf("context: %s", msg)` | |
| `raise CustomError()` | `return ErrCustom` (sentinel) or `return &CustomError{}` | |
| `try/except` | `if err != nil {}` | |
| `except SomeError as e:` | `if errors.Is(err, ErrSome)` | |
| `except SomeError as e:` (type) | `var target *SomeError; errors.As(err, &target)` | |
| `finally:` | `defer` | |
| `raise X from Y` | `fmt.Errorf("context: %w", err)` | Wrap errors |

**Error Type Example:**
```go
var (
    ErrNotFound = errors.New("not found")
    ErrConflict = errors.New("conflict")
)

type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: %s - %s", e.Field, e.Message)
}
```

### Collections and Iteration

| Python | Go | Notes |
|--------|-----|-------|
| `list[T]` | `[]T` | |
| `dict[K, V]` | `map[K]V` | |
| `set[T]` | `map[T]struct{}` | |
| `tuple[A, B]` | Named struct or multiple return values | |
| `Optional[T]` | `*T` or `T, bool` return | |
| `for item in items:` | `for _, item := range items {}` | |
| `for i, item in enumerate(items):` | `for i, item := range items {}` | |
| `[x for x in items if cond]` | Loop with append | No list comprehensions |
| `items.append(x)` | `items = append(items, x)` | |
| `len(items)` | `len(items)` | |
| `x in items` (list) | Loop or `slices.Contains()` | |
| `x in items` (dict) | `_, ok := items[x]` | |

### Types and Values

| Python | Go | Notes |
|--------|-----|-------|
| `None` | `nil` | Only for pointers, interfaces, maps, slices, channels |
| `True` / `False` | `true` / `false` | |
| `int` | `int64` | Use explicit sizes |
| `float` | `float64` | |
| `str` | `string` | |
| `bytes` | `[]byte` | |
| `UUID` | `github.com/google/uuid` | |
| `datetime` | `time.Time` | |
| `Path` | `string` | |
| `Any` | `any` (alias for `interface{}`) | Avoid when possible |

### Common Patterns

| Python | Go | Notes |
|--------|-----|-------|
| `isinstance(x, T)` | Type assertion `v, ok := x.(T)` | |
| `hasattr(obj, "method")` | Interface check | |
| `**kwargs` | Functional options pattern or config struct | |
| `*args` | Variadic `...T` | |
| Context manager `with` | `defer` for cleanup | |
| `yield` / generator | `chan T` or callback function | |
| `async def` | Regular function | Go handles concurrency differently |
| `await` | Blocking call | |
| `asyncio.gather()` | `errgroup.Group` | |

### Repository Pattern

| Python | Go |
|--------|-----|
| `Protocol` interface | `interface` |
| `Generic[T]` repository | Go generics `Repository[T]` (simplicity over complexity) |
| `QueryBuilder` | GORM query building |
| `SqlAlchemyRepository[D, E]` | GORM with generic repository pattern |
| `async with UnitOfWork` | `*gorm.DB` transaction with `defer tx.Rollback()` |
| `session_factory` | `*gorm.DB` |
| `to_domain()` / `to_db()` | Mapper functions between GORM models and domain types |

**Repository Interface Example:**
```go
type GitRepoRepository interface {
    Get(ctx context.Context, id int64) (GitRepo, error)
    Find(ctx context.Context, query Query) ([]GitRepo, error)
    Save(ctx context.Context, repo GitRepo) (GitRepo, error)
    Delete(ctx context.Context, repo GitRepo) error
    GetByRemoteURL(ctx context.Context, url string) (GitRepo, error)
}
```

### Domain Events / Task Operations

| Python | Go |
|--------|-----|
| `TaskOperation` enum | `type TaskOperation string` with constants |
| `Task` entity | Struct with same fields |
| `QueueService.enqueue_task()` | Method on `QueueService` |
| `TaskHandler.execute()` | `Handler` interface with `Execute(ctx, payload) error` |
| `TaskHandlerRegistry` | `map[TaskOperation]Handler` |

**Task Handler Example:**
```go
type Handler interface {
    Execute(ctx context.Context, payload map[string]any) error
}

type ScanCommitHandler struct {
    repoRepo   GitRepoRepository
    commitRepo GitCommitRepository
    scanner    RepositoryScanner
}

func (h *ScanCommitHandler) Execute(ctx context.Context, payload map[string]any) error {
    repoID := payload["repo_id"].(int64)
    // ... implementation
}
```

### Services

| Python | Go |
|--------|-----|
| `class XService` | Struct with dependencies |
| Constructor injection | Constructor function returning struct |
| `async def method()` | Method with `context.Context` first param |
| `ServerFactory` | Factory struct or wire for DI |

**Constructor Pattern:**
```go
type QueueService struct {
    taskRepo TaskRepository
    logger   *slog.Logger
}

func NewQueueService(taskRepo TaskRepository, logger *slog.Logger) *QueueService {
    return &QueueService{
        taskRepo: taskRepo,
        logger:   logger,
    }
}
```

### Async Patterns

| Python | Go |
|--------|-----|
| `async def` | Regular function (Go handles concurrency) |
| `await` | Blocking call |
| `asyncio.Task` | Goroutine |
| `asyncio.Event` for shutdown | `context.Context` cancellation |
| `async with` context manager | `defer` cleanup |
| `AsyncIterator` / `yield` | `chan T` |
| `asyncio.gather()` | `errgroup.Group` |

---

## Linting and Static Analysis

We use `golangci-lint` as our primary linter. All code must pass linting before considering a migration task complete.

### Running the Linter

```bash
# Run all linters
golangci-lint run

# Run on specific packages
golangci-lint run ./internal/git/...

# Run with auto-fix (where supported)
golangci-lint run --fix
```

### Linter Configuration

Create `.golangci.yml` in project root:

```yaml
run:
  timeout: 5m
  tests: true

linters:
  enable:
    - errcheck      # Unchecked errors
    - govet         # Suspicious constructs
    - staticcheck   # Static analysis
    - unused        # Unused code
    - gosimple      # Simplifications
    - ineffassign   # Ineffectual assignments
    - gofmt         # Formatting
    - goimports     # Import formatting
    - gocritic      # Opinionated checks
    - revive        # Fast, extensible linter

linters-settings:
  errcheck:
    check-type-assertions: true
    check-blank: true
  govet:
    enable-all: true
  goimports:
    local-prefixes: github.com/helixml/kodit

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

### Linting Rules

1. **Run linter after every file change**: `golangci-lint run` after creating or modifying Go files
2. **Fix errors immediately**: Don't accumulate lint errors
3. **No lint exceptions without justification**: If you must use `//nolint`, add a comment explaining why
4. **Format before commit**: Run `gofmt -w .` or `goimports -w .`

### Common Lint Fixes

| Error | Fix |
|-------|-----|
| `error return value not checked` | Add `if err != nil` check or explicitly ignore with `_ = fn()` |
| `unused variable` | Remove or use the variable |
| `should use a simple channel send/receive` | Simplify channel operations |
| `unnecessary type conversion` | Remove redundant type casts |
| `could simplify` | Apply suggested simplification |

---

## Commands

### Python (source codebase)

```bash
# Run tests
uv run pytest

# Run specific test file
uv run pytest tests/unit/test_something.py

# Type check
uv run mypy src/

# Lint
uv run ruff check src/

# Format
uv run ruff format src/

# Run application
uv run kodit serve
uv run kodit stdio
```

### Go (target codebase)

```bash
# Build
go build ./...

# Run tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint (REQUIRED after changes)
golangci-lint run

# Format
gofmt -w .
goimports -w .

# Generate (sqlc, mocks, etc.)
go generate ./...

# Run application
go run ./cmd/kodit serve
```

---

## Migration Workflow

### Task Tracking

**YOU MUST maintain the MIGRATION.md file as the source of truth for migration progress.**

After completing each migration task:

1. Mark the checkbox as complete: `- [x]`
2. Update the "Verified" checkboxes: `[x] builds [x] tests [x] lints`
3. Add the completion date if tracking timing
4. If you encountered issues, add them to the Notes section

Before starting any migration work:

1. Check MIGRATION.md for the next uncompleted task
2. Verify its dependencies are already marked complete
3. If dependencies are missing, migrate those first

### Session Start

At the beginning of each session, report:

- Current phase and context being migrated
- Next 3-5 tasks to complete
- Any blockers from previous session

### Session End

At the end of each session, update MIGRATION.md with:

- All completed checkboxes
- Any decisions made (add to Decisions Log)
- Any blockers discovered
- Notes about partially completed work

### Task Completion Criteria

A task is complete when:

- [ ] Go file created in correct location
- [ ] Test file created and passing
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` succeeds
- [ ] `golangci-lint run` passes
- [ ] Checkbox marked in MIGRATION.md

---

## Migration Rules

1. **Do not modify Python code** - The Python codebase is read-only reference material
2. **Migrate one bounded context at a time** - Start with Git Management (no dependencies), then Snippet Extraction, etc.
3. **Write tests first** - Port Python tests to Go before implementing the production code
4. **Preserve domain language** - Use the same names for entities, value objects, and operations
5. **Maintain interface boundaries** - Repository interfaces must match Python protocols
6. **Keep task operations compatible** - Task payloads must serialize identically for interop period
7. **Use dependency injection** - All services receive dependencies via constructor
8. **Context flows through** - Every public method takes `context.Context` as first parameter
9. **Errors wrap with context** - Use `fmt.Errorf("operation: %w", err)` pattern
10. **No global state** - Configuration passed explicitly, no package-level vars except errors

---

## Known Challenges

### 1. Generic Repository Pattern
Python uses `SqlAlchemyRepository[DomainEntity, DbEntity]` with dual generics. Go generics work but require careful interface design. Consider concrete implementations per entity type.

### 2. Enrichment Type Hierarchy
`EnrichmentV2` has polymorphic subtypes (architecture, development, history, usage) with further subtypes. Use interface with type/subtype discriminators and type switches.

### 3. AI Provider Abstraction (LLM + Embeddings)
Python uses `sentence-transformers` for embeddings and `LiteLLM` for LLM operations. Go has no equivalent libraries.

**Decision:** Build a single unified multi-provider abstraction that handles both:
- **Text generation** (for enrichments: summaries, cookbooks, API docs, etc.)
- **Embedding generation** (for vector search indexing)

The abstraction lives in `internal/provider/` and is used by different parts of the system:
- `internal/indexing/` uses embeddings for vector search
- `internal/enrichment/` uses text generation for semantic metadata

Providers (OpenAI, Cohere, Anthropic, etc.) implement the unified interface. A provider may support one or both capabilities.

### 4. Tree-sitter Bindings
`smacker/go-tree-sitter` exists but requires CGo. Individual language grammars need separate packages.

**Decision:** Accept the CGo dependency. Test performance early.

### 6. Async to Sync Transition
Python is fully async. Go uses goroutines differently - most code becomes synchronous with explicit goroutine spawning for workers.

### 7. Database ORM
Python uses SQLAlchemy with a generic repository pattern.

**Decision:** Use GORM (full ORM). The `QueryBuilder` with `FilterOperator` enum translates to GORM's query building capabilities.

### 8. Hierarchical Task Status
Parent/child progress tracking with real-time updates. Design channel-based notification or polling interface.

### 9. Git Library
Python uses GitPython, pygit2, and dulwich with adapters (these were options considered, not all required).

**Decision:** Use Gitea's git module (`code.gitea.io/gitea/modules/git`) which supports all required operations including shallow clone, sparse checkout, and various auth methods.

### 10. Database Migrations
Alembic migrations exist. Use `golang-migrate` or `goose`. Ensure GORM model definitions are consistent with current Python/SQLAlchemy definitions. The Go service will use the same database as Python with no schema changes required.

---

## Go Library Mapping

| Python | Go |
|--------|-----|
| SQLAlchemy | `gorm.io/gorm` |
| Alembic | `golang-migrate/migrate` |
| FastAPI | `chi` or `echo` |
| Pydantic | `go-playground/validator` |
| pydantic-settings | `kelseyhightower/envconfig` |
| Click | `spf13/cobra` |
| structlog | `log/slog` (stdlib) or `zerolog` |
| httpx | `net/http` (stdlib) |
| tree-sitter | `smacker/go-tree-sitter` (CGo) |
| bm25s | VectorChord (PostgreSQL extension) |
| GitPython/pygit2/dulwich | `code.gitea.io/gitea/modules/git` |
| litellm + sentence-transformers | Unified AI provider abstraction (`internal/provider/`) + `sashabaranov/go-openai` |
| pytest | `testing` (stdlib) + `testify` |
| fastmcp | `mark3labs/mcp-go` (Streaming HTTP) |
