# Kodit Python-to-Go Migration Checklist

## Migration Order

Bounded contexts ordered by dependencies (least dependencies first):

| Order | Context | Dependencies | Critical Path |
|-------|---------|--------------|---------------|
| 0 | Shared/Common | None | Foundation types |
| 1 | Git Management | Shared | Root aggregate |
| 2 | Task Queue & Orchestration | Shared | Infrastructure for all workflows |
| 3 | Progress Tracking | Task Queue | Needed for handler execution |
| 4 | Snippet Extraction & Indexing | Git, Task Queue | Core processing |
| 5 | Enrichment | Snippet Extraction, Task Queue | LLM integration |
| 6 | Repository Management | Git, Task Queue | Lifecycle operations |
| 7 | Code Search | Snippet Extraction, Enrichment | Query interface |
| 8 | API Gateway | All contexts | External interface |

---

## 0. Shared/Common Types

### Domain Layer

#### Value Objects

- [x] `src/kodit/domain/value_objects.py` → `internal/domain/value.go`

  Description: Core value objects (LanguageMapping, PaginationParams, FilterOperator, QueuePriority, TaskOperation enum)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/value_objects.py:605-697` → `internal/queue/operation.go`

  Description: TaskOperation enum (30+ operations) and PrescribedOperations choreography definitions
  Dependencies: None
  Verified: [x] builds [x] tests pass

#### Error Types

- [x] `src/kodit/domain/errors.py` → `internal/domain/errors.go`

  Description: Domain error types (EmptySourceError, etc.)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/api/client/exceptions.py` → `internal/api/errors.go`

  Description: API error hierarchy (KoditAPIError, AuthenticationError, ServerError)
  Dependencies: None
  Verified: [x] builds [x] tests pass

#### Configuration

- [x] `src/kodit/config.py` → `internal/config/config.go`

  Description: AppContext with all settings (DataDir, DBUrl, endpoints, search config, etc.)
  Dependencies: None
  Verified: [x] builds [x] tests pass

#### Logging

- [x] `src/kodit/log.py` → `internal/log/logger.go`

  Description: Structured logging setup with correlation IDs
  Dependencies: config
  Verified: [x] builds [x] tests pass

#### AI Provider Abstraction

- [x] `src/kodit/infrastructure/providers/litellm_provider.py` + `src/kodit/infrastructure/embedding/embedding_providers/` → `internal/provider/provider.go`

  Description: Unified AI provider interface supporting both text generation (for enrichments) and embedding generation (for vector search)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] → `internal/provider/openai.go`

  Description: OpenAI provider implementation (supports both text generation and embeddings)
  Dependencies: provider interface, sashabaranov/go-openai
  Verified: [x] builds [x] tests pass

- [ ] → `internal/provider/[additional].go`

  Description: Additional providers (Cohere, Anthropic, etc.) added as needed
  Dependencies: provider interface
  Verified: [ ] builds [ ] tests pass

### Infrastructure Layer

#### Database Foundation

- [x] `src/kodit/database.py` → `internal/database/database.go`

  Description: Database connection, migration runner, session factory
  Dependencies: config
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/sqlalchemy/unit_of_work.py` → `internal/database/transaction.go`

  Description: Transaction wrapper (UnitOfWork pattern)
  Dependencies: database
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/sqlalchemy/query.py` → `internal/database/query.go`

  Description: QueryBuilder with FilterOperator support (using GORM query building)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/sqlalchemy/repository.py` → `internal/database/repository.go`

  Description: Generic GORM repository base with Go generics (to_domain/to_db patterns)
  Dependencies: query, unit_of_work
  Verified: [x] builds [x] tests pass

#### Database Migrations

- [ ] `src/kodit/migrations/` → `migrations/`

  Description: All Alembic migrations converted to golang-migrate format
  Dependencies: database
  Verified: [ ] builds [ ] migrations run

### Tests

- [x] `tests/conftest.py` → `internal/testutil/fixtures.go`

  Description: Test fixtures, database setup, common test utilities
  Dependencies: database, config
  Verified: [x] builds [x] tests pass

---

## 1. Git Management Context

### Domain Layer

#### Entities

- [x] `src/kodit/domain/entities/git.py:GitRepo` → `internal/git/repo.go`

  Description: GitRepo aggregate root (id, remote_url, working_copy, tracking_config)
  Dependencies: value objects
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/entities/git.py:GitCommit` → `internal/git/commit.go`

  Description: GitCommit entity (sha, repo_id, message, author, timestamp)
  Dependencies: GitRepo
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/entities/git.py:GitBranch` → `internal/git/branch.go`

  Description: GitBranch entity (repo_id, name, head_commit_sha)
  Dependencies: GitRepo
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/entities/git.py:GitTag` → `internal/git/tag.go`

  Description: GitTag entity (repo_id, name, commit_sha)
  Dependencies: GitRepo
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/entities/git.py:GitFile` → `internal/git/file.go`

  Description: GitFile entity (commit_sha, path, language)
  Dependencies: GitCommit
  Verified: [x] builds [x] tests pass

#### Value Objects

- [x] `src/kodit/domain/entities/git.py:WorkingCopy` → `internal/git/working_copy.go`

  Description: Immutable value object (path, uri)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/entities/git.py:TrackingConfig` → `internal/git/tracking_config.go`

  Description: Immutable config (branch, tag, commit to track)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/entities/git.py:RepositoryScanResult` → `internal/git/scan_result.go`

  Description: Immutable scan output (branches, commits, files, tags)
  Dependencies: GitBranch, GitCommit, GitFile, GitTag
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/entities/git.py:Author` → `internal/git/author.go`

  Description: Immutable author (name, email)
  Dependencies: None
  Verified: [x] builds [x] tests pass

#### Repository Interfaces

- [x] `src/kodit/domain/protocols.py:GitRepoRepository` → `internal/git/repository.go`

  Description: GitRepoRepository interface (CRUD + GetByRemoteURL)
  Dependencies: GitRepo
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/protocols.py:GitCommitRepository` → `internal/git/repository.go`

  Description: GitCommitRepository interface (CRUD + GetByRepoAndSHA)
  Dependencies: GitCommit
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/protocols.py:GitBranchRepository` → `internal/git/repository.go`

  Description: GitBranchRepository interface (CRUD + GetByName)
  Dependencies: GitBranch
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/protocols.py:GitTagRepository` → `internal/git/repository.go`

  Description: GitTagRepository interface
  Dependencies: GitTag
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/protocols.py:GitFileRepository` → `internal/git/repository.go`

  Description: GitFileRepository interface (CRUD + DeleteByCommitSHA)
  Dependencies: GitFile
  Verified: [x] builds [x] tests pass

#### Domain Services

- [x] `src/kodit/domain/services/git_repository_service.py` → `internal/git/scanner.go`

  Description: GitRepositoryScanner service (scan repo, extract metadata)
  Dependencies: All git entities
  Verified: [x] builds [x] tests pass

### Application Layer

- [ ] (No application services in this context - handlers are in Task Queue context)

### Infrastructure Layer

#### Repository Implementations

- [x] `src/kodit/infrastructure/sqlalchemy/git_repo_repository.py` → `internal/git/postgres/repo_repository.go`

  Description: PostgreSQL GitRepoRepository implementation
  Dependencies: GitRepo, database
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/sqlalchemy/git_commit_repository.py` → `internal/git/postgres/commit_repository.go`

  Description: PostgreSQL GitCommitRepository implementation
  Dependencies: GitCommit, database
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/sqlalchemy/git_branch_repository.py` → `internal/git/postgres/branch_repository.go`

  Description: PostgreSQL GitBranchRepository implementation
  Dependencies: GitBranch, database
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/sqlalchemy/git_tag_repository.py` → `internal/git/postgres/tag_repository.go`

  Description: PostgreSQL GitTagRepository implementation
  Dependencies: GitTag, database
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/sqlalchemy/git_file_repository.py` → `internal/git/postgres/file_repository.go`

  Description: PostgreSQL GitFileRepository implementation
  Dependencies: GitFile, database
  Verified: [x] builds [x] tests pass

#### Database Entities

- [x] `src/kodit/infrastructure/sqlalchemy/entities.py:GitRepo` → `internal/git/postgres/entity.go`

  Description: Database entity mappings for all Git types
  Dependencies: database
  Verified: [x] builds [x] tests pass

#### Mappers

- [x] → `internal/git/postgres/mapper.go`

  Description: Entity mappers (RepoMapper, CommitMapper, BranchMapper, TagMapper, FileMapper)
  Dependencies: database entities, domain types
  Verified: [x] builds [x] tests pass

#### Git Adapters

- [x] `src/kodit/infrastructure/git/` → `internal/git/adapter.go`

  Description: Git library adapter interface
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/cloning/git/git_python_adaptor.py` → `internal/git/gitadapter/gogit.go`

  Description: go-git adapter implementation (using github.com/go-git/go-git/v5 instead of Gitea modules)
  Dependencies: Git adapter interface
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/cloning/cloner.py` → `internal/git/cloner.go`

  Description: RepositoryCloner service
  Dependencies: Git adapter
  Verified: [x] builds [x] tests pass

#### Ignore Patterns

- [x] `src/kodit/infrastructure/ignore/ignore_pattern_provider.py` → `internal/git/ignore.go`

  Description: Gitignore pattern matching
  Dependencies: Git adapter
  Verified: [x] builds [x] tests pass

### Tests

- [x] `tests/unit/domain/entities/test_git.py` → `internal/git/entity_test.go`

  Description: Unit tests for Git entities and value objects
  Dependencies: All git entities
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/infrastructure/sqlalchemy/test_git_*_repository.py` → `internal/git/postgres/repository_test.go`

  Description: Repository integration tests
  Dependencies: All git repositories
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/infrastructure/git/` → `internal/git/gitadapter/gogit_test.go`

  Description: Git adapter tests
  Dependencies: Git adapters
  Verified: [x] builds [x] tests pass

---

## 2. Task Queue & Orchestration Context

### Domain Layer

#### Entities

- [x] `src/kodit/domain/entities/__init__.py:Task` → `internal/queue/task.go`

  Description: Task entity (id, dedup_key, type, priority, payload, created_at)
  Dependencies: TaskOperation
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/entities/__init__.py:TaskStatus` → `internal/queue/status.go`

  Description: TaskStatus entity with state machine (STARTED → IN_PROGRESS → COMPLETED/FAILED/SKIPPED)
  Dependencies: Task
  Verified: [x] builds [x] tests pass

#### Repository Interfaces

- [x] `src/kodit/domain/protocols.py:TaskRepository` → `internal/queue/repository.go`

  Description: TaskRepository interface (CRUD + dequeue + priority ordering)
  Dependencies: Task
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/protocols.py:TaskStatusRepository` → `internal/queue/repository.go`

  Description: TaskStatusRepository interface
  Dependencies: TaskStatus
  Verified: [x] builds [x] tests pass

### Application Layer

#### Services

- [x] `src/kodit/application/services/queue_service.py` → `internal/queue/service.go`

  Description: QueueService (enqueue_task, enqueue_tasks with choreography)
  Dependencies: TaskRepository, TaskOperation
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/services/indexing_worker_service.py` → `internal/queue/worker.go`

  Description: IndexingWorkerService (poll loop, task processing, graceful shutdown)
  Dependencies: QueueService, TaskHandlerRegistry
  Verified: [x] builds [x] tests pass

#### Handler Infrastructure

- [x] `src/kodit/application/handlers/__init__.py:TaskHandler` → `internal/queue/handler.go`

  Description: TaskHandler interface (Execute method)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/handlers/registry.py` → `internal/queue/registry.go`

  Description: TaskHandlerRegistry (register/lookup handlers by operation)
  Dependencies: TaskHandler, TaskOperation
  Verified: [x] builds [x] tests pass

### Infrastructure Layer

#### Repository Implementations

- [x] `src/kodit/infrastructure/sqlalchemy/task_repository.py` → `internal/queue/postgres/task_repository.go`

  Description: PostgreSQL TaskRepository implementation
  Dependencies: Task, database
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/sqlalchemy/task_status_repository.py` → `internal/queue/postgres/status_repository.go`

  Description: PostgreSQL TaskStatusRepository implementation
  Dependencies: TaskStatus, database
  Verified: [x] builds [x] tests pass

#### Database Entities

- [x] `src/kodit/infrastructure/sqlalchemy/entities.py:Task` → `internal/queue/postgres/entity.go`

  Description: Database entity mappings for Task and TaskStatus (includes mapper.go)
  Dependencies: database
  Verified: [x] builds [x] tests pass

### Tests

- [x] `tests/unit/domain/entities/test_task.py` → `internal/queue/task_test.go`

  Description: Unit tests for Task entity
  Dependencies: Task
  Verified: [x] builds [x] tests pass

- [x] → `internal/queue/status_test.go`

  Description: Unit tests for TaskStatus entity
  Dependencies: TaskStatus
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/application/services/test_queue_service.py` → `internal/queue/service_test.go`

  Description: QueueService unit tests (requires fake repository)
  Dependencies: QueueService
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/application/services/test_indexing_worker_service.py` → `internal/queue/worker_test.go`

  Description: Worker service tests (requires fake repository)
  Dependencies: Worker
  Verified: [x] builds [x] tests pass

---

## 3. Progress Tracking Context

### Domain Layer

#### Interfaces

- [x] `src/kodit/domain/tracking/trackable.py` → `internal/tracking/trackable.go`

  Description: Trackable interface and TrackableType enum
  Dependencies: None
  Verified: [x] builds [x] tests pass

#### Value Objects

- [x] `src/kodit/domain/tracking/status.py` → `internal/tracking/status.go`

  Description: RepositoryStatusSummary, IndexStatus value objects
  Dependencies: Trackable
  Verified: [x] builds [x] tests pass

### Application Layer

#### Services

- [x] `src/kodit/application/services/reporting.py:ProgressTracker` → `internal/tracking/tracker.go`

  Description: ProgressTracker with observer pattern (notify_subscribers)
  Dependencies: ReportingModule
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/services/reporting.py:ReportingModule` → `internal/tracking/reporter.go`

  Description: ReportingModule interface (observer)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/services/task_status_query_service.py` → `internal/tracking/query.go`

  Description: TaskStatusQueryService for progress queries
  Dependencies: TaskStatusRepository
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/services/trackable_resolution_service.py` → `internal/tracking/resolver.go`

  Description: TrackableResolutionService (resolve branch/tag/commit references)
  Dependencies: Git repositories
  Verified: [x] builds [x] tests pass

### Infrastructure Layer

#### Reporting Modules

- [x] `src/kodit/infrastructure/reporting/logging_module.py` → `internal/tracking/logging_reporter.go`

  Description: LoggingReportingModule subscriber
  Dependencies: ReportingModule, logger
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/reporting/db_module.py` → `internal/tracking/db_reporter.go`

  Description: DBProgressReportingModule subscriber
  Dependencies: ReportingModule, TaskStatusRepository
  Verified: [x] builds [x] tests pass

- [ ] `src/kodit/infrastructure/reporting/telemetry_module.py` → `internal/tracking/telemetry_reporter.go`

  Description: TelemetryProgressReportingModule subscriber
  Dependencies: ReportingModule, analytics client
  Verified: [ ] builds [ ] tests pass

### Tests

- [x] `tests/unit/domain/tracking/` → `internal/tracking/tracking_test.go`

  Description: Tracking domain tests
  Dependencies: Trackable, status types
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/application/services/test_reporting.py` → `internal/tracking/tracker_test.go`

  Description: ProgressTracker tests
  Dependencies: ProgressTracker
  Verified: [x] builds [x] tests pass

---

## 4. Snippet Extraction & Indexing Context

### Domain Layer

#### Entities

- [x] `src/kodit/domain/entities/git.py:SnippetV2` → `internal/indexing/snippet.go`

  Description: SnippetV2 aggregate (content-addressed by SHA256, with code, language, type)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/entities/__init__.py:CommitIndex` → `internal/indexing/commit_index.go`

  Description: CommitIndex aggregate (snippets for a commit with status)
  Dependencies: SnippetV2
  Verified: [x] builds [x] tests pass

#### Repository Interfaces

- [x] `src/kodit/domain/protocols.py:SnippetRepositoryV2` → `internal/indexing/repository.go`

  Description: SnippetRepository, CommitIndexRepository, BM25Repository, VectorSearchRepository interfaces
  Dependencies: SnippetV2
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/services/bm25_service.py:BM25Repository` → `internal/indexing/repository.go`

  Description: BM25Repository interface (search, index operations)
  Dependencies: SnippetV2
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/services/embedding_service.py:VectorSearchRepository` → `internal/indexing/repository.go`

  Description: VectorSearchRepository interface (similarity search)
  Dependencies: SnippetV2
  Verified: [x] builds [x] tests pass

#### Domain Services

- [x] `src/kodit/domain/services/bm25_service.py:BM25DomainService` → `internal/indexing/bm25_service.go`

  Description: BM25 indexing domain service
  Dependencies: BM25Repository
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/services/embedding_service.py:EmbeddingDomainService` → `internal/indexing/embedding_service.go`

  Description: Embedding/vector indexing domain service
  Dependencies: VectorSearchRepository, EmbeddingProvider
  Verified: [x] builds [x] tests pass

### Application Layer

#### Handlers

- [x] `src/kodit/application/handlers/snippets/extract_snippets_handler.py` → `internal/queue/handler/extract_snippets.go`

  Description: EXTRACT_SNIPPETS_FOR_COMMIT handler
  Dependencies: Slicer, SnippetRepository
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/handlers/indexing/create_bm25_index_handler.py` → `internal/queue/handler/create_bm25.go`

  Description: CREATE_BM25_INDEX_FOR_COMMIT handler
  Dependencies: BM25DomainService
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/handlers/indexing/create_embeddings_handler.py` → `internal/queue/handler/create_embeddings.go`

  Description: CREATE_CODE_EMBEDDINGS_FOR_COMMIT handler
  Dependencies: EmbeddingDomainService
  Verified: [x] builds [x] tests pass

### Infrastructure Layer

#### Repository Implementations

- [x] `src/kodit/infrastructure/sqlalchemy/snippet_v2_repository.py` → `internal/indexing/postgres/snippet_repository.go`

  Description: PostgreSQL SnippetRepositoryV2 implementation
  Dependencies: SnippetV2, database
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/bm25/vectorchord_bm25_repository.py` → `internal/indexing/bm25/vectorchord_repository.go`

  Description: VectorChord BM25 implementation (primary BM25 store, batch-only updates)
  Dependencies: BM25Repository, GORM
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/embedding/vectorchord_vector_search_repository.py` → `internal/indexing/vector/vectorchord_repository.go`

  Description: VectorChord vector search repository implementation
  Dependencies: VectorSearchRepository, provider.Embedder
  Verified: [x] builds [x] tests pass

#### Slicing (AST Parsing)

- [x] `src/kodit/infrastructure/slicing/language_config.py` → `internal/indexing/slicer/config.go`

  Description: Language configuration (supported languages, tree-sitter grammars)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/slicing/language_analyzer.py` → `internal/indexing/slicer/analyzer.go`

  Description: LanguageAnalyzer interface (extract functions, classes, etc.)
  Dependencies: tree-sitter
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/slicing/analyzers/*.py` → `internal/indexing/slicer/analyzers/`

  Description: Language-specific analyzers (Python, Go, JavaScript, TypeScript, etc.)
  Dependencies: LanguageAnalyzer
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/slicing/slicer.py` → `internal/indexing/slicer/slicer.go`

  Description: Main Slicer service (orchestrates AST parsing)
  Dependencies: LanguageAnalyzer, tree-sitter
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/slicing/ast_analyzer.py` → `internal/indexing/slicer/ast.go`

  Description: AST traversal utilities
  Dependencies: tree-sitter
  Verified: [x] builds [x] tests pass

#### Embedding Service (uses shared AI Provider)

- [x] `src/kodit/infrastructure/embedding/embedding_providers/` → `internal/indexing/embedding_service.go`

  Description: Embedding service that uses the shared AI provider abstraction (`internal/provider/`) for vector generation. Note: Implemented in `internal/indexing/embedding_service.go` (not a separate subdirectory).
  Dependencies: internal/provider
  Verified: [x] builds [x] tests pass

#### Database Entities

- [x] `src/kodit/infrastructure/sqlalchemy/entities.py:SnippetV2` → `internal/indexing/postgres/entity.go`

  Description: Database entity mappings for CommitIndex, Snippet, and related association tables
  Dependencies: database
  Verified: [x] builds [x] tests pass

### Tests

- [x] `tests/unit/domain/entities/test_snippet.py` → `internal/indexing/snippet_test.go`

  Description: SnippetV2, CommitIndex, BM25Service, EmbeddingService entity tests
  Dependencies: SnippetV2
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/infrastructure/slicing/` → `internal/indexing/slicer/slicer_test.go`

  Description: Slicer and analyzer tests
  Dependencies: Slicer
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/infrastructure/bm25/` → `internal/indexing/bm25/vectorchord_repository_test.go`

  Description: BM25 repository tests
  Dependencies: BM25 repositories
  Verified: [x] builds [x] tests pass

- [x] → `internal/indexing/vector/vectorchord_repository_test.go`

  Description: Vector search repository tests
  Dependencies: VectorSearchRepository
  Verified: [x] builds [x] tests pass

---

## 5. Enrichment Context

### Domain Layer

#### Entities

- [x] `src/kodit/domain/enrichments/enrichment.py` → `internal/enrichment/enrichment.go`

  Description: EnrichmentV2 interface and base types (Type, Subtype, EntityTypeKey)
  Dependencies: None
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/enrichments/architecture/` → `internal/enrichment/architecture.go`

  Description: ArchitectureEnrichment subtypes (physical, database_schema)
  Dependencies: EnrichmentV2
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/enrichments/development/` → `internal/enrichment/development.go`

  Description: DevelopmentEnrichment subtypes (snippet, snippet_summary, example, example_summary)
  Dependencies: EnrichmentV2
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/enrichments/history/` → `internal/enrichment/history.go`

  Description: HistoryEnrichment subtypes (commit_description)
  Dependencies: EnrichmentV2
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/enrichments/usage/` → `internal/enrichment/usage.go`

  Description: UsageEnrichment subtypes (cookbook, api_docs)
  Dependencies: EnrichmentV2
  Verified: [x] builds [x] tests pass

#### Value Objects

- [x] `src/kodit/domain/enrichments/enrichment.py:EnrichmentAssociation` → `internal/enrichment/association.go`

  Description: EnrichmentAssociation (links enrichments to snippets)
  Dependencies: EnrichmentV2
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/enrichments/enrichment.py:CommitEnrichmentAssociation` → `internal/enrichment/association.go`

  Description: CommitEnrichmentAssociation (links enrichments to commits)
  Dependencies: EnrichmentV2
  Verified: [x] builds [x] tests pass

#### Repository Interfaces

- [x] `src/kodit/domain/protocols.py:EnrichmentV2Repository` → `internal/enrichment/repository.go`

  Description: EnrichmentV2Repository interface
  Dependencies: EnrichmentV2
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/protocols.py:EnrichmentAssociationRepository` → `internal/enrichment/repository.go`

  Description: EnrichmentAssociationRepository interface
  Dependencies: EnrichmentAssociation
  Verified: [x] builds [x] tests pass

### Application Layer

#### Handlers

- [x] `src/kodit/application/handlers/enrichments/` → `internal/queue/handler/enrichment/`

  Description: All enrichment task handlers (CREATE_*_ENRICHMENT operations)
  Dependencies: EnrichmentV2Repository, LLMProvider
  Verified: [x] builds [x] tests pass

#### Services

- [x] `src/kodit/domain/services/physical_architecture_service.py` → `internal/enrichment/physical_architecture.go`

  Description: PhysicalArchitectureService (system structure discovery)
  Dependencies: LLMProvider
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/domain/services/cookbook_context_service.py` → `internal/enrichment/cookbook_context.go`

  Description: CookbookContextService (usage guide generation)
  Dependencies: LLMProvider, SnippetRepository
  Verified: [x] builds [x] tests pass

- [x] → `internal/enrichment/query.go`

  Description: EnrichmentQueryService (check for existing enrichments by commit)
  Dependencies: EnrichmentRepository, AssociationRepository
  Verified: [x] builds [x] tests pass

### Infrastructure Layer

#### Repository Implementations

- [x] `src/kodit/infrastructure/sqlalchemy/enrichment_v2_repository.py` → `internal/enrichment/postgres/enrichment_repository.go`

  Description: PostgreSQL EnrichmentV2Repository implementation
  Dependencies: EnrichmentV2, database
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/infrastructure/sqlalchemy/enrichment_association_repository.py` → `internal/enrichment/postgres/association_repository.go`

  Description: PostgreSQL EnrichmentAssociationRepository implementation
  Dependencies: EnrichmentAssociation, database
  Verified: [x] builds [x] tests pass

#### Enrichment Service (uses shared AI Provider)

Note: The LLM provider abstraction lives in `internal/provider/` (shared). The enrichment context uses it for text generation.

- [x] `src/kodit/infrastructure/enricher/local_enricher.py` uses shared provider from `internal/provider/`

  Description: Enricher service uses the shared AI provider abstraction for text generation (summaries, cookbooks, API docs, etc.)
  Dependencies: internal/provider
  Verified: [x] builds [x] tests pass

#### Enricher

- [x] `src/kodit/infrastructure/enricher/local_enricher.py` → `internal/enrichment/enricher.go`

  Description: Enricher service (orchestrates text generation calls via shared AI provider)
  Dependencies: internal/provider
  Verified: [x] builds [x] tests pass

#### Example Extraction

- [x] `src/kodit/infrastructure/example_extraction/` → `internal/enrichment/example/`

  Description: Example extraction from documentation (CodeBlock, Discovery, MarkdownParser, RstParser)
  Dependencies: DocumentationParser
  Verified: [x] builds [x] tests pass

#### Database Entities

- [x] `src/kodit/infrastructure/sqlalchemy/entities.py:EnrichmentV2` → `internal/enrichment/postgres/entity.go`

  Description: Database entity mappings for enrichments (includes mapper.go)
  Dependencies: database
  Verified: [x] builds [x] tests pass

### Tests

- [x] `tests/unit/domain/enrichments/` → `internal/enrichment/enrichment_test.go`

  Description: Enrichment entity and hierarchy tests
  Dependencies: All enrichment types
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/infrastructure/enricher/` → `internal/enrichment/enricher_test.go`

  Description: Enricher service tests
  Dependencies: Enricher
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/infrastructure/providers/` → `internal/provider/provider_test.go`

  Description: LLM provider tests (in internal/provider/)
  Dependencies: LLMProvider
  Verified: [x] builds [x] tests pass

---

## 6. Repository Management Context

### Domain Layer

#### Entities

- [x] `src/kodit/domain/entities/__init__.py:Source` → `internal/repository/source.go`

  Description: Source entity (repository with WorkingCopy reference)
  Dependencies: WorkingCopy
  Verified: [x] builds [x] tests pass

### Application Layer

#### Handlers

- [x] `src/kodit/application/handlers/repository/clone_repository_handler.py` → `internal/queue/handler/clone_repository.go`

  Description: CLONE_REPOSITORY handler
  Dependencies: RepositoryCloner, GitRepoRepository
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/handlers/repository/sync_repository_handler.py` → `internal/queue/handler/sync_repository.go`

  Description: SYNC_REPOSITORY handler
  Dependencies: Git adapter, GitRepoRepository
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/handlers/repository/delete_repository_handler.py` → `internal/queue/handler/delete_repository.go`

  Description: DELETE_REPOSITORY handler
  Dependencies: GitRepoRepository, cleanup services
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/handlers/commits/scan_commit_handler.py` → `internal/queue/handler/scan_commit.go`

  Description: SCAN_COMMIT handler
  Dependencies: GitRepositoryScanner, GitCommitRepository
  Verified: [x] builds [x] tests pass

#### Services

- [x] `src/kodit/application/services/repository_query_service.py` → `internal/repository/query.go`

  Description: RepositoryQueryService (read-only queries for repos)
  Dependencies: GitRepoRepository
  Verified: [x] builds [x] tests pass

- [x] `src/kodit/application/services/repository_sync_service.py` → `internal/repository/sync.go`

  Description: RepositorySyncService (sync orchestration)
  Dependencies: QueueService, GitRepoRepository
  Verified: [x] builds [x] tests pass

### Tests

- [x] `tests/unit/application/handlers/repository/` → `internal/queue/handler/repository_test.go`

  Description: Repository handler tests
  Dependencies: All repository handlers
  Verified: [x] builds [x] tests pass

- [x] `tests/unit/application/services/test_repository_*_service.py` → `internal/repository/service_test.go`

  Description: Repository service tests
  Dependencies: Repository services
  Verified: [x] builds [x] tests pass

---

## 7. Code Search Context

### Domain Layer

#### Value Objects

- [ ] `src/kodit/application/services/code_search_application_service.py:MultiSearchRequest` → `internal/search/request.go`

  Description: MultiSearchRequest (query, filters, pagination)
  Dependencies: SnippetSearchFilters
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/application/services/code_search_application_service.py:MultiSearchResult` → `internal/search/result.go`

  Description: MultiSearchResult (snippets, enrichments, scores)
  Dependencies: SnippetV2, EnrichmentV2
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/infrastructure/indexing/fusion_service.py:FusionRequest` → `internal/search/fusion.go`

  Description: FusionRequest (BM25 + vector inputs)
  Dependencies: None
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/infrastructure/indexing/fusion_service.py:FusionResult` → `internal/search/fusion.go`

  Description: FusionResult (combined scores via reciprocal rank fusion)
  Dependencies: None
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/domain/value_objects.py:SnippetSearchFilters` → `internal/search/filters.go`

  Description: SnippetSearchFilters (language, author, dates, repo, enrichment types)
  Dependencies: None
  Verified: [ ] builds [ ] tests pass

### Application Layer

#### Services

- [ ] `src/kodit/application/services/code_search_application_service.py` → `internal/search/service.go`

  Description: CodeSearchApplicationService (orchestrates hybrid search)
  Dependencies: BM25Repository, VectorSearchRepository, FusionService
  Verified: [ ] builds [ ] tests pass

### Infrastructure Layer

#### Services

- [ ] `src/kodit/infrastructure/indexing/fusion_service.py` → `internal/search/fusion_service.go`

  Description: FusionService (reciprocal rank fusion algorithm)
  Dependencies: None
  Verified: [ ] builds [ ] tests pass

### Tests

- [ ] `tests/unit/application/services/test_code_search_application_service.py` → `internal/search/service_test.go`

  Description: Search service tests
  Dependencies: CodeSearchApplicationService
  Verified: [ ] builds [ ] tests pass

- [ ] `tests/unit/infrastructure/indexing/test_fusion_service.py` → `internal/search/fusion_test.go`

  Description: Fusion algorithm tests
  Dependencies: FusionService
  Verified: [ ] builds [ ] tests pass

---

## 8. API Gateway Context

### Infrastructure Layer

#### Server Setup

- [ ] `src/kodit/app.py` → `internal/api/server.go`

  Description: HTTP server setup, lifespan management, middleware registration
  Dependencies: All services
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/cli.py` → `cmd/kodit/main.go`

  Description: CLI entry point (serve, stdio, version commands)
  Dependencies: Server, config
  Verified: [ ] builds [ ] tests pass

#### Middleware

- [ ] `src/kodit/infrastructure/api/middleware.py` → `internal/api/middleware/`

  Description: Request logging, correlation ID, error handling middleware
  Dependencies: Logger
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/infrastructure/api/dependencies.py` → `internal/api/dependencies.go`

  Description: Dependency injection for handlers
  Dependencies: ServerFactory
  Verified: [ ] builds [ ] tests pass

#### Routers (Endpoints)

- [ ] `src/kodit/infrastructure/api/v1/routers/repositories.py` → `internal/api/v1/repositories.go`

  Description: /api/v1/repositories endpoints
  Dependencies: RepositoryQueryService, QueueService
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/infrastructure/api/v1/routers/commits.py` → `internal/api/v1/commits.go`

  Description: /api/v1/commits endpoints
  Dependencies: GitCommitRepository
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/infrastructure/api/v1/routers/search.py` → `internal/api/v1/search.go`

  Description: /api/v1/search endpoints
  Dependencies: CodeSearchApplicationService
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/infrastructure/api/v1/routers/enrichments.py` → `internal/api/v1/enrichments.go`

  Description: /api/v1/enrichments endpoints
  Dependencies: EnrichmentV2Repository
  Verified: [ ] builds [ ] tests pass

- [ ] `src/kodit/infrastructure/api/v1/routers/queue.py` → `internal/api/v1/queue.go`

  Description: /api/v1/queue endpoints
  Dependencies: TaskRepository, TaskStatusQueryService
  Verified: [ ] builds [ ] tests pass

#### Schemas (DTOs)

- [ ] `src/kodit/infrastructure/api/v1/schemas/` → `internal/api/v1/dto/`

  Description: Request/response DTOs for all endpoints
  Dependencies: Domain entities
  Verified: [ ] builds [ ] tests pass

#### MCP Server (Required for MVP)

- [ ] `src/kodit/mcp.py` → `internal/mcp/server.go`

  Description: MCP (Model Context Protocol) server via Streaming HTTP (not STDIO). Basic tool registration only.
  Dependencies: All services, mark3labs/mcp-go
  Verified: [ ] builds [ ] tests pass

### Factory

- [ ] `src/kodit/application/factories/server_factory.py` → `internal/factory/server.go`

  Description: ServerFactory (dependency injection, handler registration)
  Dependencies: All repositories, services, handlers
  Verified: [ ] builds [ ] tests pass

### Tests

- [ ] `tests/unit/infrastructure/api/` → `internal/api/api_test.go`

  Description: API endpoint tests
  Dependencies: All routers
  Verified: [ ] builds [ ] tests pass

- [ ] `tests/integration/` → `internal/api/integration_test.go`

  Description: Full integration tests
  Dependencies: Server, database
  Verified: [ ] builds [ ] tests pass

- [ ] `tests/e2e/` → `test/e2e/`

  Description: End-to-end tests
  Dependencies: Full system
  Verified: [ ] builds [ ] tests pass

---

## Blockers & Decisions

| ID | Category | Issue | Options | Decision | Status |
|----|----------|-------|---------|----------|--------|
| B1 | AI Providers | No pure-Go equivalents for LiteLLM or sentence-transformers | 1) Separate abstractions 2) Unified abstraction 3) Keep Python service | Unified AI provider abstraction in `internal/provider/` handling both text generation and embeddings | Resolved |
| B2 | tree-sitter | CGo required for go-tree-sitter | 1) Accept CGo 2) External parsing service 3) Alternative parser | Accept CGo dependency | Resolved |
| B4 | Git Libraries | Python uses 3 libraries (GitPython, pygit2, dulwich) | 1) go-git 2) git2go 3) gitea git module | Use go-git (`github.com/go-git/go-git/v5`) - pure Go, no CGO required | Resolved |
| B5 | BM25 Search | bm25s is Python-specific | 1) bleve full-text search 2) Custom BM25 impl 3) VectorChord | VectorChord (already in use), batch-only updates | Resolved |
| B6 | Database ORM | SQLAlchemy generic repository pattern | 1) sqlc (generated) 2) sqlx (manual) 3) GORM (ORM) | GORM (full ORM) | Resolved |
| B7 | Task Payload Compat | Must serialize identically for interop period | Define JSON schema for all payloads | N/A - no interop period, migrating one context at a time | Resolved |
| D1 | Package Structure | Flat vs nested packages | Follow CLAUDE.md structure | Structure not final, repositories live separately (`internal/repository/`) | Resolved |
| D2 | Error Handling | Sentinel vs typed errors | Mix: sentinels for common, typed for context | Wrap errors only at boundaries, use jsonapi.org/format#errors for API | Resolved |
| D3 | Generics | Use Go generics for Repository[T]? | Evaluate per-entity vs generic interfaces | Use Go generics, simplicity over type safety | Resolved |

---

## Notes

<!-- Running log of migration notes, discoveries, and learnings -->

### Session Log

| Date | Note |
|------|------|
| 2026-02-02 | Session 1: Completed 8 tasks in Phase 0 (Shared/Common Types). Created Go module, domain value objects, queue operations, domain errors, API errors, config, logging, and AI provider interface + OpenAI implementation. All tests passing, linting clean. |
| 2026-02-02 | Session 2: Completed 5 more tasks in Phase 0 Infrastructure Layer. Created database.go (GORM connection), transaction.go (UnitOfWork pattern), query.go (QueryBuilder with FilterOperator), repository.go (generic Repository[D,E]), and testutil/fixtures.go. Database migrations deferred (no schema changes required). Then completed 14 tasks in Phase 1 Git Management Domain Layer: all entities (Repo, Commit, Branch, Tag, File), all value objects (WorkingCopy, TrackingConfig, Author, ScanResult), and all repository interfaces. |
| 2026-02-02 | Session 3: Completed Git Management Infrastructure Layer. Created: adapter.go (Git Adapter interface), scanner.go (GitRepositoryScanner service), postgres/entity.go (GORM entities), postgres/mapper.go (domain<->entity mappers), postgres/repo_repository.go, postgres/commit_repository.go, postgres/branch_repository.go, postgres/tag_repository.go, postgres/file_repository.go, gitadapter/gogit.go (go-git implementation using github.com/go-git/go-git/v5), and cloner.go (RepositoryCloner service). All tests pass, linting clean. Decision: Use go-git instead of Gitea modules - pure Go, no CGO required. |
| 2026-02-02 | Session 4: Completed final Git Management tasks. Created: ignore.go (IgnorePattern for gitignore + .noindex patterns), postgres/repository_test.go (comprehensive integration tests for all 5 repository implementations), gitadapter/gogit_test.go (adapter tests covering all operations). Added GetByCommitAndPath method to FileRepository. Git Management Context is now 100% complete. All tests pass, linting clean. |
| 2026-02-02 | Session 5: Completed Task Queue & Orchestration Context (12/14 tasks). Created: task.go (Task entity with dedup key generation), status.go (TaskStatus with state machine), repository.go (TaskRepository and TaskStatusRepository interfaces), handler.go (Handler interface), registry.go (Registry for operation->handler mapping), service.go (QueueService), worker.go (Worker with poll loop and graceful shutdown), postgres/entity.go (GORM entities), postgres/mapper.go (mappers), postgres/task_repository.go, postgres/status_repository.go. Tests: task_test.go, status_test.go. Remaining: service_test.go and worker_test.go (require fakes). All tests pass, linting clean. |
| 2026-02-02 | Session 6: Completed Task Queue & Orchestration Context (14/14 tasks - 100%). Created: fake.go (FakeTaskRepository, FakeTaskStatusRepository, FakeHandler for testing), service_test.go (QueueService tests), worker_test.go (Worker tests). Phase 2 is now complete. All tests pass, linting clean. |
| 2026-02-02 | Session 7: Completed Progress Tracking Context (10/11 tasks - 91%). Created: trackable.go (Trackable value object with ReferenceType), status.go (RepositoryStatusSummary with StatusSummaryFromTasks), tracker.go (Tracker with observer pattern), reporter.go (Reporter interface), query.go (QueryService), resolver.go (Resolver for trackable->commits), logging_reporter.go (LoggingReporter), db_reporter.go (DBReporter), tracking_test.go, tracker_test.go. Telemetry reporter deferred (requires analytics client). All tests pass, linting clean. Note: Commit.ParentCommitSHA not yet implemented in Git context, so full parent traversal in Resolver returns single commit. |
| 2026-02-02 | Session 8: Started Snippet Extraction & Indexing Context (8/19 tasks). Created: snippet.go (Snippet aggregate, content-addressed by SHA256), commit_index.go (CommitIndex aggregate with status machine), repository.go (SnippetRepository, CommitIndexRepository, BM25Repository, VectorSearchRepository interfaces), bm25_service.go (BM25Service with validation), embedding_service.go (EmbeddingService with deduplication), postgres/entity.go (GORM entities for CommitIndex, Snippet, associations), postgres/mapper.go (domain<->entity mappers), postgres/commit_index_repository.go. Tests: snippet_test.go, commit_index_test.go, bm25_service_test.go, embedding_service_test.go. All tests pass, linting clean. |
| 2026-02-02 | Session 9: Continued Snippet Extraction & Indexing Context (13/19 tasks - 68%). Created: postgres/snippet_repository.go (SnippetRepository implementation with content-addressed deduplication, commit associations, search filters), bm25/vectorchord_repository.go (VectorChord BM25 implementation using PostgreSQL extensions), vector/vectorchord_repository.go (VectorChord vector search using provider.Embedder for embeddings). Tests: postgres/snippet_repository_test.go, bm25/vectorchord_repository_test.go, vector/vectorchord_repository_test.go. Remaining: slicer components (config, analyzer, analyzers/, slicer, ast), embedding service, and task handlers. All tests pass, linting clean. |
| 2026-02-02 | Session 10: Completed Slicer/AST Parsing components (19/19 tasks - 100% for Snippet Extraction core). Created: slicer/config.go (LanguageConfig with 10 supported languages: Python, Go, Java, C, C++, Rust, JavaScript, TypeScript, TSX, C#), slicer/analyzer.go (Analyzer interface with FunctionDefinition, ClassDefinition, TypeDefinition value objects), slicer/ast.go (Walker for AST traversal, CallGraph for dependency tracking), slicer/analyzers/ (language-specific implementations: base.go, python.go, golang.go, javascript.go, typescript.go, java.go, c.go, cpp.go, rust.go, csharp.go, factory.go), slicer/slicer.go (main Slicer service with file parsing, definition extraction, call graph building, snippet generation). Tests: slicer_test.go. Tree-sitter CGo dependency added (github.com/smacker/go-tree-sitter). Remaining in context: embedding service and task handlers. All tests pass, linting clean. |
| 2026-02-02 | Session 11: Completed Snippet Extraction & Indexing Application Layer handlers (22/22 tasks - 100%). Created: handler/extract_snippets.go (EXTRACT_SNIPPETS_FOR_COMMIT handler with slicer integration, progress tracking), handler/create_bm25.go (CREATE_BM25_INDEX_FOR_COMMIT handler), handler/create_embeddings.go (CREATE_CODE_EMBEDDINGS_FOR_COMMIT handler with embedding deduplication). Tests: handler/handler_test.go with comprehensive tests for BM25 and embedding handlers. Note: embedding_service.go already existed in indexing/ (not a separate subdirectory). Phase 4 Snippet Extraction & Indexing Context is now 100% complete. All tests pass, linting clean. |
| 2026-02-02 | Session 12: Started Enrichment Context (13/18 tasks - 72%). Created domain layer: enrichment.go (Enrichment with Type/Subtype/EntityTypeKey, immutable value object), architecture.go, development.go, history.go, usage.go (factory functions and type predicates for all enrichment subtypes), association.go (Association and SnippetSummaryLink value objects), repository.go (EnrichmentRepository and AssociationRepository interfaces). Created infrastructure layer: postgres/entity.go (GORM entities), postgres/mapper.go (domain<->entity mappers), postgres/enrichment_repository.go, postgres/association_repository.go. Created enricher.go (ProviderEnricher service using TextGenerator for LLM calls). Tests: enrichment_test.go (comprehensive tests for all types). Remaining: task handlers, PhysicalArchitectureService, CookbookContextService, example extraction. All tests pass, linting clean. |
| 2026-02-02 | Session 13: Completed Enrichment Context (18/18 tasks - 100%). Created: handler/enrichment/ package with all 8 enrichment task handlers (util.go, create_summary.go, commit_description.go, architecture_discovery.go, database_schema.go, cookbook.go, api_docs.go, extract_examples.go, example_summary.go), query.go (EnrichmentQueryService for idempotency checks), physical_architecture.go (Docker Compose analysis, service detection, port inference), cookbook_context.go (README extraction, package manifest reading, example discovery), example/ package (code_block.go, discovery.go, parser.go with Markdown and RST parsers), enricher_test.go. Tests: handler/enrichment/handler_test.go with comprehensive fakes, example/example_test.go. Phase 5 Enrichment Context is now 100% complete. All tests pass, linting clean. |
| 2026-02-02 | Session 14: Completed Repository Management Context (8/8 tasks - 100%). Created: source.go (Source entity wrapping git.Repo with status tracking), clone_repository.go (CLONE_REPOSITORY handler), sync_repository.go (SYNC_REPOSITORY handler with branch scanning and commit scan queueing), delete_repository.go (DELETE_REPOSITORY handler with full cascade delete), scan_commit.go (SCAN_COMMIT handler with file extraction), query.go (QueryService for repository reads with RepositorySummary), sync.go (SyncService for add/sync/delete orchestration). Tests: source_test.go, repository_test.go (handler helper tests), service_test.go (QueryService and SyncService tests). Phase 6 Repository Management Context is now 100% complete. All tests pass, linting clean. |

### Architecture Decisions

- **Unified AI provider**: Single abstraction in `internal/provider/` handles both text generation (enrichments) and embedding generation (vector search). Providers implement one or both capabilities.
- **No interop period**: Python and Go services will NOT run simultaneously. Migration is one context at a time with immediate cutover.
- **Same database**: Go service uses the same database as Python with no schema changes required.
- **MCP required for MVP**: Model Context Protocol via Streaming HTTP is required.
- **API compatibility**: Must maintain /api/v1/ compatibility with existing clients. OpenAPI spec exists in Python codebase.
- **Manual DI**: No dependency injection framework (wire, fx). Use manual construction.
- **All languages supported**: MVP must support all existing languages for snippet extraction.
- **All enrichment types**: All ~8 enrichment subtypes required for MVP (one LLM call per enrichment).
- **No rollback strategy**: Migration is forward-only.

### Known Differences

- **Progress tracking**: Start with API polling (not real-time). WebSocket/SSE not required.
- **Monitoring**: Basic logging only. No metrics, tracing, or dashboards.
- **Configuration**: Environment variables for backwards compatibility. Future: API-loaded config stored in database.

### Testing Strategy

- Focus on e2e tests, no unit test coverage requirements.
- No parity validation between Python and Go implementations.
- Use fakes, not mocks.

### Performance Observations

- No latency SLOs for API endpoints.
- No throughput requirements for task queue worker.
- No memory or CPU constraints.
- BM25 score parity not required (approximate is acceptable).
