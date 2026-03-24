# Design: Per-Repository Pipeline Configuration

## Architecture Overview

`Pipeline` and `PipelineStep` are first-class domain entities with their own database tables. A pipeline is linked to a repository and contains an ordered list of steps. Each step knows its kind, its configuration, and which other steps it depends on. This makes it straightforward to add new built-in operations or data-driven custom steps (e.g. arbitrary enrichment prompts) without touching the domain model.

---

## Domain Layer

### New package: `domain/pipeline`

#### `pipeline_step.go`

```go
type StepKind string

const (
    StepKindBuiltIn StepKind = "built-in"
    // StepKindCustomEnrichment StepKind = "custom-enrichment"  // future
)

type PipelineStep struct {
    id         int64
    pipelineID int64
    kind       StepKind
    operation  task.Operation // non-empty for built-in steps
    config     map[string]any // reserved for future custom steps
    dependsOn  []int64        // IDs of other PipelineSteps in the same pipeline
    position   int            // display/insertion order
}
```

Constructors: `NewBuiltInStep(pipelineID int64, op task.Operation, dependsOn []int64, position int) PipelineStep`, `ReconstructStep(...)`.

Getters: `ID`, `PipelineID`, `Kind`, `Operation`, `Config`, `DependsOn`, `Position`.

#### `pipeline.go`

```go
type Pipeline struct {
    id           int64
    repositoryID int64
    steps        []PipelineStep
    createdAt    time.Time
    updatedAt    time.Time
}
```

**Constructors:**
- `NewPipeline(repositoryID int64, steps []PipelineStep) Pipeline`
- `ReconstructPipeline(id, repositoryID int64, steps []PipelineStep, createdAt, updatedAt time.Time) Pipeline`

**Methods:**
- `Steps() []PipelineStep` — ordered by `position`.
- `Operations() []task.Operation` — returns the operation string for every built-in step, in position order.
- `HasOperation(op task.Operation) bool`
- `Validate() error` — checks: all dependency IDs exist within this pipeline; core steps are present; no cycles.

**Preset factories** (package-level functions, not methods, so they can be used without an existing pipeline):
- `DefaultPipeline(repositoryID int64, hasTextProvider bool) Pipeline`
- `RAGOnlyPipeline(repositoryID int64) Pipeline`
- `FullPipeline(repositoryID int64) Pipeline`

These build the step list from the static dependency map (see below) and assign positions and `dependsOn` IDs after inserting steps.

**Dependency map** (package-level `var`):

```go
var builtInDependencies = map[task.Operation][]task.Operation{
    task.OperationSyncRepository:                            {task.OperationCloneRepository},
    task.OperationScanCommit:                                {task.OperationCloneRepository},
    task.OperationExtractSnippetsForCommit:                  {task.OperationScanCommit},
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
}

var coreOperations = []task.Operation{
    task.OperationCloneRepository,
    task.OperationSyncRepository,
    task.OperationScanCommit,
    task.OperationExtractSnippetsForCommit,
}
```

#### `store.go`

```go
type Store interface {
    FindByRepo(ctx context.Context, repositoryID int64) (Pipeline, error)
    Save(ctx context.Context, pipeline Pipeline) (Pipeline, error)
    Delete(ctx context.Context, pipeline Pipeline) error
}
```

### `domain/repository/repository.go`

No change to the `Repository` struct. `Pipeline` is a separate aggregate; the relationship is expressed by `repositoryID` on the pipeline, not by embedding inside `Repository`.

---

## Persistence Layer

### New models (`infrastructure/persistence/models.go`)

```go
type PipelineModel struct {
    ID           int64          `gorm:"primaryKey;autoIncrement"`
    RepositoryID int64          `gorm:"column:repository_id;uniqueIndex"`
    Repo         RepositoryModel `gorm:"foreignKey:RepositoryID;constraint:OnDelete:CASCADE"`
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

func (PipelineModel) TableName() string { return "pipelines" }

type PipelineStepModel struct {
    ID         int64         `gorm:"primaryKey;autoIncrement"`
    PipelineID int64         `gorm:"column:pipeline_id;index"`
    Pipeline   PipelineModel `gorm:"foreignKey:PipelineID;constraint:OnDelete:CASCADE"`
    Kind       string        `gorm:"column:kind;size:64"`
    Operation  string        `gorm:"column:operation;size:255"`
    Config     string        `gorm:"column:config;type:text"` // JSON, empty for built-in
    DependsOn  string        `gorm:"column:depends_on;type:text"` // JSON array of step IDs
    Position   int           `gorm:"column:position"`
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

func (PipelineStepModel) TableName() string { return "pipeline_steps" }
```

Add both to `AutoMigrate`. No changes to `RepositoryModel`.

### New store: `infrastructure/persistence/pipeline_store.go`

Implements `pipeline.Store`. `FindByRepo` loads the `PipelineModel` by `repository_id`, then loads all `PipelineStepModel` rows for that pipeline, decodes `DependsOn` JSON, and maps to `pipeline.Pipeline` via a mapper.

---

## Application Layer

### Integration point: queue enqueue calls

The application service (or handler) already loads the repository. Add a load of the pipeline:

```go
pl, err := svc.Pipelines.FindByRepo(ctx, repoID)
// ...
ops := pl.Operations()
// Filter to the action-specific candidate list (scan vs rescan vs index):
candidates := prescribed.ScanAndIndexCommit()
enqueueOps := intersect(candidates, ops)
queue.EnqueueOperations(ctx, repoID, enqueueOps)
```

`PrescribedOperations` still defines the canonical sequence for each action context. The pipeline's `Operations()` is the inclusion filter.

### Repository creation (`application/service/repository.go`)

`RepositoryAddParams` gains `PipelinePreset string`. After saving the `Repository`, the service resolves the preset to a `Pipeline` using the appropriate factory function, saves it via `PipelineStore`, then enqueues tasks. This is atomic with respect to task enqueueing.

---

## API Layer

### Repository creation

`RepositoryCreateAttributes` DTO gains:
```go
PipelinePreset string `json:"pipeline_preset,omitempty"`
```
Valid values: `"rag-only"`, `"full"`, `"default"` (default when omitted). Returns 400 for unknown values or `"full"` without a text provider.

### New pipeline resource routes

```
GET    /api/v1/repositories/{id}/pipeline          → GetPipeline
PUT    /api/v1/repositories/{id}/pipeline/steps    → ReplaceSteps
DELETE /api/v1/repositories/{id}/pipeline/steps/{step_id}  → RemoveStep
```

**GET** returns the full pipeline with steps:
```json
{
  "id": 1,
  "repository_id": 42,
  "steps": [
    { "id": 10, "kind": "built-in", "operation": "kodit.repository.clone", "depends_on": [], "position": 0 },
    { "id": 11, "kind": "built-in", "operation": "kodit.commit.extract_snippets", "depends_on": [10], "position": 2 }
  ]
}
```

**PUT /steps** replaces the full step list (re-validates dependencies). Used for bulk edits.

**DELETE /steps/{step_id}** removes one step; returns 400 if other steps depend on it.

---

## Key Design Decisions

**Why proper entities and not a JSON column?** A JSON blob on `git_repos` cannot represent future custom steps that carry their own configuration (e.g. an enrichment prompt, a target entity type). Real rows enable querying, per-step config, independent lifecycle management, and extension without schema changes.

**Why a separate `Pipeline` aggregate, not fields on `Repository`?** The pipeline has its own identity, its own lifecycle (can be replaced wholesale), and its own set of steps. Embedding it inside `Repository` would violate the single-responsibility principle and make the repository aggregate heavier. The `repositoryID` foreign key is the link.

**Why store `dependsOn` as step IDs rather than operation strings?** Step IDs are stable within a pipeline; operation strings are an implementation detail of built-in steps. Future custom steps have no operation string to reference. Storing IDs keeps the dependency graph self-contained and portable.

**Why keep `PrescribedOperations`?** It still defines the canonical execution sequence within an action context (scan vs rescan vs index). `Pipeline.Operations()` acts as the per-repository inclusion filter. Removing `PrescribedOperations` would require duplicating sequencing logic into the pipeline entity.

**Why move preset to repository creation?** Setting the pipeline after creation introduces a race: the worker can begin the clone task before the pipeline is saved, running with the wrong steps. Creating the pipeline in the same service call as the repository — before task enqueueing — eliminates this race.

**Migration for existing repositories:** Existing repos have no `pipelines` row. The `FindByRepo` method returns `ErrNotFound`; callers fall back to `DefaultPipeline(repoID, hasTextProvider)` and save it lazily on first access.
