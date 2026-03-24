# Design: Per-Repository Pipeline Configuration

## Architecture Overview

The pipeline config follows the same pattern as `ChunkingConfig`: a domain value object attached to `Repository`, persisted as columns on `git_repos`, and exposed via a dedicated API sub-resource.

---

## Domain Layer

### `domain/repository/pipeline_config.go`

```go
type PipelineConfig struct {
    steps []task.Operation
}
```

**Constructors** (mirrors existing `PrescribedOperations` factories):
- `DefaultPipelineConfig(hasTextProvider bool) PipelineConfig`
- `RAGOnlyPipelineConfig() PipelineConfig`
- `FullPipelineConfig() PipelineConfig`
- `ReconstructPipelineConfig(steps []task.Operation) (PipelineConfig, error)` — used by persistence layer; validates dependencies.

**Methods:**
- `Steps() []task.Operation` — full ordered list for storage/serialization.
- `Filter(ops []task.Operation) []task.Operation` — returns only ops present in config; preserves order. Used by callers that already have a candidate list (e.g. `prescribed.ScanAndIndexCommit()`).
- `Contains(op task.Operation) bool`
- `Validate() error` — checks all dependency constraints and that core steps are present.

**Dependency map** — defined as a package-level `var` inside `pipeline_config.go`:

```go
var stepDependencies = map[task.Operation][]task.Operation{
    task.OperationCreateBM25IndexForCommit:                  {task.OperationExtractSnippetsForCommit},
    task.OperationCreateCodeEmbeddingsForCommit:             {task.OperationExtractSnippetsForCommit},
    task.OperationCreateSummaryEmbeddingsForCommit:          {task.OperationCreateCodeEmbeddingsForCommit},
    task.OperationExtractExamplesForCommit:                  {task.OperationScanCommit},
    task.OperationCreateExampleCodeEmbeddingsForCommit:      {task.OperationExtractExamplesForCommit},
    task.OperationCreateSummaryEnrichmentForCommit:          {task.OperationExtractExamplesForCommit},
    task.OperationCreateExampleSummaryForCommit:             {task.OperationCreateSummaryEnrichmentForCommit},
    task.OperationCreateExampleSummaryEmbeddingsForCommit:   {task.OperationCreateExampleSummaryForCommit},
    task.OperationCreatePublicAPIDocsForCommit:              {task.OperationExtractSnippetsForCommit},
    task.OperationCreateArchitectureEnrichmentForCommit:     {task.OperationExtractSnippetsForCommit},
    task.OperationCreateCommitDescriptionForCommit:          {task.OperationScanCommit},
    task.OperationCreateDatabaseSchemaForCommit:             {task.OperationExtractSnippetsForCommit},
    task.OperationCreateCookbookForCommit:                   {task.OperationExtractSnippetsForCommit},
    task.OperationGenerateWikiForCommit:                     {task.OperationExtractSnippetsForCommit},
    task.OperationSyncRepository:                            {task.OperationCloneRepository},
    task.OperationScanCommit:                                {task.OperationCloneRepository},
    task.OperationExtractSnippetsForCommit:                  {task.OperationScanCommit},
}

var coreSteps = []task.Operation{
    task.OperationCloneRepository,
    task.OperationSyncRepository,
    task.OperationScanCommit,
    task.OperationExtractSnippetsForCommit,
}
```

### `domain/repository/repository.go`

Add field `pipelineConfig PipelineConfig`. Add getter `PipelineConfig()` and mutator `WithPipelineConfig(PipelineConfig) Repository`. Update `NewRepository` to accept `hasTextProvider bool` or set a default pipeline config post-construction, and update `ReconstructRepository` signature.

---

## Persistence Layer

### `RepositoryModel` (infrastructure/persistence/models.go)

Add one column:

```go
PipelineSteps string `gorm:"column:pipeline_steps;type:text;default:'[]'"`
```

Store as a JSON-encoded `[]string` (the `Operation.String()` values). GORM AutoMigrate adds the column automatically; existing rows get `'[]'` which the mapper treats as "default pipeline for server config".

### Mapper

In `RepositoryMapper.ToDomain()`: if `PipelineSteps` is empty/`[]`, call `DefaultPipelineConfig(hasTextProvider)` using the server's current capability. Otherwise call `ReconstructPipelineConfig(decoded steps)`.

In `RepositoryMapper.ToModel()`: JSON-encode `repo.PipelineConfig().Steps()` to string.

> Note: The mapper needs access to `hasTextProvider`. Pass it as a constructor argument to the mapper, the same way other infrastructure config is threaded through. The repository store constructs the mapper, so the store constructor receives this bool from the application layer.

---

## Application Layer

### Integration point: queue enqueue calls

Currently, callers build an operation list from `PrescribedOperations`:
```go
ops := prescribed.ScanAndIndexCommit()
queue.EnqueueOperations(ctx, repoID, ops)
```

Replace with:
```go
ops := repo.PipelineConfig().Filter(prescribed.ScanAndIndexCommit())
queue.EnqueueOperations(ctx, repoID, ops)
```

`PrescribedOperations` still determines the "universe" for a given action context; `PipelineConfig.Filter` narrows it to what the repo has enabled. This change applies in the commit handler and wherever tasks are enqueued for a specific action (scan, index, rescan).

---

## API Layer

### New routes (infrastructure/api/v1/repositories.go)

```
GET  /api/v1/repositories/{id}/config/pipeline        → GetPipelineConfig
PUT  /api/v1/repositories/{id}/config/pipeline        → UpdatePipelineConfig
POST /api/v1/repositories/{id}/config/pipeline/init   → InitPipelineConfig
```

### GET response
```json
{
  "steps": [
    "kodit.repository.clone",
    "kodit.repository.sync",
    "kodit.commit.scan",
    "kodit.commit.extract_snippets",
    "kodit.commit.create_bm25_index",
    "kodit.commit.create_code_embeddings"
  ]
}
```

### PUT request/response
Request body: `{"steps": ["kodit.commit.extract_snippets", ...]}`.
Validates via `PipelineConfig.Validate()`. Returns updated config on success; 400 with error message on invalid input.

### POST /init request
```json
{ "preset": "rag-only" }
```
Valid values: `"rag-only"`, `"full"`, `"default"`. Returns 400 if `full` is requested and no text provider is configured.

---

## Key Design Decisions

**Why store as JSON text, not a separate table?** A pipeline config is a small ordered list (≤20 items). A join table would add schema complexity for no query benefit — we always load the full list with the repository.

**Why keep `PrescribedOperations`?** It remains useful as a factory for generating sensible defaults and for generating the "candidate" operation list per action type (scan vs rescan vs index). `PipelineConfig.Filter` then narrows that candidate list. This avoids duplicating the sequencing logic.

**Why validate on PUT (not cascade-remove)?** Explicit rejection makes the caller aware of the dependency graph. Silently removing dependents could cause surprise data loss. The error message names the missing dependency, so the client knows what to add back or also remove.

**Migration for existing rows:** Existing repos get `pipeline_steps = '[]'`. The mapper interprets this as "use the server default", so behaviour is unchanged until the user explicitly sets a pipeline via the API.
