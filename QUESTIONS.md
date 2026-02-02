# Kodit Migration Questions

Outstanding questions that need resolution before or during the Python-to-Go migration.

## Technical Blockers

These are the open blockers identified in the migration checklist that need decisions:

### B1: Embedding Provider Strategy

**Context:** Python uses `sentence-transformers` with local PyTorch models. No pure-Go equivalent exists.

1. ONNX Runtime via CGo
2. External embedding API (OpenAI, Cohere, etc.)
3. Keep Python service for embeddings during transition

**Answer**: Build multi-provider abstraction layer. This should be the same as the enrichment provider abstraction.

---

### B2: Tree-sitter Strategy

**Context:** AST parsing uses tree-sitter. `smacker/go-tree-sitter` requires CGo.

**Options:**
1. Accept CGo dependency
2. External parsing service
3. Alternative parser (per-language)

**Answer:** 1. Accept CGo dependency

---

### B3: LLM Provider Strategy

**Context:** Python uses LiteLLM which wraps 100+ LLM providers.

**Options:**
1. `go-openai` for OpenAI-compatible APIs only
2. Build multi-provider abstraction layer
3. Call Python service for LLM operations

**Answer**: 2. Build multi-provider abstraction layer. This should be the same as the embedding provider abstraction.

---

### B4: Git Library Consolidation

**Context:** Python uses three libraries (GitPython, pygit2, dulwich) with adapters.

**Options:**
1. Consolidate on `go-git` (pure Go)
2. Use `git2go` (libgit2 bindings, CGo) for features go-git lacks
3. Hybrid approach

**Questions:**
- Why does Python need three libraries? What features does each provide?
  A: It doesn't need three libraries. It needs one library that supports all the features we need. These three were options we considered.
- Does `go-git` support all required operations (shallow clone, sparse checkout, auth methods)?
  A: No it doesn't. Please use 	giteagit "code.gitea.io/gitea/modules/git"

---

### B5: BM25 Search Implementation

**Context:** Python uses `bm25s` which is Python-specific.

**Options:**
1. `blevesearch/bleve` (full-text search engine)
2. Custom BM25 implementation
3. PostgreSQL full-text search
4. External service (Elasticsearch, Meilisearch)

**Questions:**
- Is the current BM25 index stored in files or database?
  A: In vectorchord
- What is the index size for typical repositories?
  A: Not sure.
- Are there custom tokenization or stemming requirements?
  A: No.
- Is real-time index updates required or batch-only acceptable?
  A: Batch-only.

---

### B6: Database ORM/Query Builder

**Context:** Python uses SQLAlchemy with a generic repository pattern.

**Options:**
1. `sqlc` (generated type-safe queries)
2. `sqlx` (manual SQL with struct scanning)
3. `GORM` (full ORM)

Answer: 3. GORM (full ORM)

---

## Migration Strategy Questions

### Interoperability Period

- Will Python and Go services run simultaneously during migration?
  A: No.
- How will traffic be routed between Python and Go during cutover?
  A: We will not be cutting over immediately. We will be migrating one context at a time.   
- Is a gradual rollout (percentage-based) planned?
  A: No.
- What is the rollback strategy if Go migration fails?
  A: We will not be rolling back. 

### Data Migration

- Will the Go service use the same database as Python?
  A: Yes.
- Are there any schema changes required for Go?
  A: No.
- How will existing Alembic migrations be converted to golang-migrate?
  A: Make sure the GORM representations are consistent with the current Python definitions.
- What is the strategy for migrating local file indexes (BM25, etc.)?
  A: None.

### Testing Strategy

- What test coverage level is required before each context is considered "migrated"?
  A: None, focus on e2e tests.
- How will parity between Python and Go be validated?
  A: No need.
- Will there be comparison testing (same input → same output)?
  A: No.
- How will integration tests run during the mixed Python/Go period?
  A: None.

---

## Architecture Questions

### Package Structure (D1)

- The CLAUDE.md proposes a structure, but is it final?
  A: No.
- Should repositories live with their domain (`internal/git/repository.go`) or separately (`internal/repository/git.go`)?
  A: Separately.
- Where should shared types live (`internal/domain/` vs package-local)?
  A: Shared types should live in the domain package.

### Error Handling (D2)

- Should all errors be wrapped with context, or only at boundaries?
  A: Only at boundaries.
- Is there a standard for error codes (numeric, string)?
  A: No.
- How should validation errors be structured for API responses?
  A: jsonapi.orr/format#errors

### Generics (D3)

- Should `Repository[T]` use Go generics or concrete interfaces per entity?
  A: Generics.
- What is the trade-off evaluation criteria (type safety vs complexity)?
  A: Simplity is more important.
- Are there existing patterns in Go codebases the team has worked with?
  A: No.

### Dependency Injection

- Is a DI framework (wire, fx) preferred, or manual construction?
  A: Manual
- How should service lifetimes be managed (singleton, per-request)?
  A: Depends on the service.
- What is the initialization order for services with circular dependencies?
  A: Depends on the service.

---

## API and Protocol Questions

### REST API

- Will the API remain at `/api/v1/` or bump to `/api/v2/` for Go?
  A: /api/v1/.
- Are there breaking changes expected in request/response schemas?
  A: No.
- How will API versioning be handled long-term?
  A: No versioning.

### MCP Protocol

- Is MCP (Model Context Protocol) required for MVP?
  A: Yes.
- What is the current MCP usage pattern (STDIO only, or network)?
  A: Streaming HTTP.
- Are there MCP-specific features beyond basic tool registration?
  A: No.

### Client Compatibility

- Are there existing API clients that must remain compatible?
  A: Yes, mcp and api.
- Is there an OpenAPI spec that defines the contract?
  A: yes in the python codebase.
- What is the deprecation timeline for Python-only features?
  A: Immediate, although I'm hoping we can keep some kind of backwards compatibility.

---

## Operational Questions

### Monitoring and Observability

- What metrics are currently collected (Prometheus, StatsD, etc.)?
  A: None. Just basic logging.
- What tracing is in place (OpenTelemetry, Jaeger)?
  A: None.
- Are there existing dashboards that need Go equivalents?
  A: None.

### Deployment

- What is the target deployment environment (K8s, ECS, bare metal)?
  A: All of the above.
- Are there container size or startup time requirements?
  A: No.
- How will configuration be managed (env vars, files, secrets)?
  A: Environment variables for backwards compatibility, but dynamic API loaded for storage in the database is the future.

### Performance Requirements

- What are the latency SLOs for API endpoints?
  A: None.
- What throughput is required for the task queue worker?
  A: None.
- Are there memory or CPU constraints?
  A: No.

---

## Feature-Specific Questions

### Snippet Extraction

- How many languages must be supported at MVP?
  A: All existing languages.
- Are there language-specific edge cases documented?
  A: No.
- What is the expected snippet extraction rate (snippets/minute)?
  A: None.

### Enrichments

- Which enrichment types are used most frequently?
  A: Snippets.
- Are all enrichment types (~8 subtypes) required for MVP?
  A: yes.
- What is the typical LLM token usage per enrichment?
  A: One llm call per enrichment.

### Search

- What is the expected query latency target?
  A: None.
- How important is exact BM25 score parity vs approximate?
  A: None.
- Are there reranking or filtering features beyond basic fusion?
  A: No.

### Progress Tracking

- Is real-time progress required, or is polling acceptable?
  A: Interesting idea. Real time would be nice, but start with API polling for backwards compatibility.
- What is the update frequency for progress events?
  A: None.
- Are there WebSocket or SSE requirements?
  A: No.

---