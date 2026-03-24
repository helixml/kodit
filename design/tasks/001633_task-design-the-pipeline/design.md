# Design: Pipeline Domain — Entities, Store, and Database Schema

## Patterns Found in This Codebase

- **Immutable domain objects**: all mutations return a new value (see `Repository.WithUpstreamURL`, `WithChunkingConfig`); `updatedAt` bumped on write
- **`Reconstruct*` for persistence**: `New*` is for user-created entities (validates, sets timestamps); `Reconstruct*` is for loading from DB (no validation, sets ID)
- **`EntityMapper[D, E]` pattern**: `ToDomain(E) D` and `ToModel(D) E` — one mapper per entity
- **`database.Repository[D, E]` embedding**: all stores embed this; exposes `DB(ctx)`, `Mapper()`, `Find`, `FindOne`, `Count`, `Exists`, `DeleteBy`
- **Options via `repository.WithCondition`**: domain packages define typed options (e.g. `WithRepoID`) on top of the generic query builder in `domain/repository/query.go`
- **GORM AutoMigrate + postMigrate**: `AutoMigrate` registers all GORM models; `postMigrate` manually creates FK constraints when GORM's auto-generation is unreliable (see GORM bug #7693 for composite PKs)
- **JOIN overrides**: when a store needs JOINs or preloads beyond what the generic `Repository` provides, it overrides `Find`/`FindOne` (see `EnrichmentStore`)
- **Fail fast**: return `fmt.Errorf("...: %w", err)`, never log-and-continue

## File Layout

```
domain/pipeline/
  pipeline.go        # Step and Pipeline entities, Pipeline behaviours
  catalog.go         # Catalog with step kind registry and preset builders
  store.go           # PipelineStore interface
  options.go         # WithRepoID alias + WithPipelineID

infrastructure/persistence/
  pipeline_models.go  # PipelineModel, PipelineStepModel, PipelineStepDependencyModel
  pipeline_mapper.go  # PipelineMapper — reconstructs full aggregate from models
  pipeline_store.go   # PipelineStore implementation

# db.go: add pipeline models to AutoMigrate call and add FK constraints in postMigrate
```

## Domain Entities

### Step

```go
// domain/pipeline/pipeline.go

type Step struct {
    id        int64
    kind      string
    config    map[string]any
    dependsOn []int64
}

func NewStep(kind string, config map[string]any) Step
func ReconstructStep(id int64, kind string, config map[string]any, dependsOn []int64) Step

func (s Step) ID() int64
func (s Step) Kind() string
func (s Step) Config() map[string]any   // returns copy
func (s Step) DependsOn() []int64       // returns copy
```

### Pipeline (aggregate root)

```go
type Pipeline struct {
    id        int64
    repoID    int64
    steps     []Step
    createdAt time.Time
    updatedAt time.Time
}

func NewPipeline(repoID int64) Pipeline
func ReconstructPipeline(id, repoID int64, steps []Step, createdAt, updatedAt time.Time) Pipeline

// Queries
func (p Pipeline) ID() int64
func (p Pipeline) RepoID() int64
func (p Pipeline) Steps() []Step
func (p Pipeline) CreatedAt() time.Time
func (p Pipeline) UpdatedAt() time.Time
func (p Pipeline) Has(kind string) bool
func (p Pipeline) Step(kind string) (Step, bool)
func (p Pipeline) Roots() []Step
func (p Pipeline) DependentsOf(stepID int64) []Step
func (p Pipeline) TopologicalOrder() []Step       // Kahn's algorithm; panics if cycle (call Validate first)

// Mutations — return new Pipeline
func (p Pipeline) With(step Step) (Pipeline, error)   // errors if any dependsOn ID missing
func (p Pipeline) Without(stepID int64) Pipeline      // removes step + transitive dependents

// Validation
func (p Pipeline) Validate() error  // cycle check, dangling dependsOn, duplicate kinds
```

**TopologicalOrder implementation**: Kahn's algorithm — build in-degree map, queue nodes with zero in-degree, pop and reduce in-degrees, append to result. If result length < len(steps), there is a cycle (but Validate catches this first).

**Without implementation**: BFS/DFS from the removed step through `DependentsOf` to collect all transitive dependents, then filter them out.

### Catalog

```go
// domain/pipeline/catalog.go

type stepDef struct {
    requires []string   // kinds that must precede this step
}

type Catalog struct {
    defs map[string]stepDef
}

func NewCatalog() Catalog   // populates defs for all commit-level operations

func (c Catalog) Build(repoID int64, kinds []string) (Pipeline, error)
func (c Catalog) RAGOnly(repoID int64) Pipeline   // extract_snippets, bm25_index, code_embeddings
func (c Catalog) Full(repoID int64) Pipeline      // all known commit-level step kinds
```

**Catalog step definitions** (derived from `domain/task/operation.go`; structural ops clone/sync/scan/rescan excluded):

| Kind | Requires |
|---|---|
| `extract_snippets` | — |
| `bm25_index` | `extract_snippets` |
| `code_embeddings` | `extract_snippets` |
| `examples` | `extract_snippets` |
| `example_code_embeddings` | `examples` |
| `summary_enrichment` | `examples` |
| `summary_embeddings` | `summary_enrichment` |
| `example_summary` | `examples`, `summary_enrichment` |
| `example_summary_embeddings` | `example_summary` |
| `architecture_enrichment` | `extract_snippets` |
| `public_api_docs` | `extract_snippets` |
| `commit_description` | `extract_snippets` |
| `database_schema` | `extract_snippets` |
| `cookbook` | `extract_snippets` |
| `wiki` | `extract_snippets` |

`Build` iterates the requested kinds in topological order of the catalog DAG (so that step IDs are assigned in dependency order), creates `NewStep` for each, then wires dependsOn IDs from kind→ID lookup. Returns error if any required kind is missing from the requested set.

## Store Interface

```go
// domain/pipeline/store.go

type Store interface {
    Find(ctx context.Context, options ...repository.Option) ([]Pipeline, error)
    FindOne(ctx context.Context, options ...repository.Option) (Pipeline, error)
    Save(ctx context.Context, p Pipeline) (Pipeline, error)
    Delete(ctx context.Context, p Pipeline) error
}
```

Note: `Count` is excluded from the interface — it is not needed for the pipeline use case. Use the embedded `database.Repository` directly if needed.

## Options

```go
// domain/pipeline/options.go

// Option is an alias for the shared query option type.
type Option = repository.Option

// WithRepoID filters by repo_id — reuses repository.WithRepoID directly.
// Re-exported for convenience so callers only import domain/pipeline.
func WithRepoID(id int64) Option { return repository.WithRepoID(id) }

// WithPipelineID filters by id (the pipeline's own PK).
func WithPipelineID(id int64) Option { return repository.WithID(id) }
```

## Database Models

```go
// infrastructure/persistence/pipeline_models.go

type PipelineModel struct {
    ID        int64          `gorm:"primaryKey;autoIncrement"`
    RepoID    int64          `gorm:"column:repo_id;uniqueIndex;not null"`
    Repo      RepositoryModel `gorm:"foreignKey:RepoID;references:ID;constraint:OnDelete:CASCADE"`
    Steps     []PipelineStepModel `gorm:"foreignKey:PipelineID"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
func (PipelineModel) TableName() string { return "pipelines" }

type PipelineStepModel struct {
    ID           int64          `gorm:"primaryKey;autoIncrement"`
    PipelineID   int64          `gorm:"column:pipeline_id;index;not null"`
    Pipeline     PipelineModel  `gorm:"foreignKey:PipelineID;references:ID;constraint:OnDelete:CASCADE"`
    Kind         string         `gorm:"column:kind;size:100;not null"`
    Config       datatypes.JSON `gorm:"column:config;default:'{}'"`
    Dependencies []PipelineStepDependencyModel `gorm:"foreignKey:StepID"`
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
func (PipelineStepModel) TableName() string { return "pipeline_steps" }

// pipeline_step_dependencies — pure join table; no autoIncrement PK.
// composite PK triggers GORM bug #7693, so FKs use constraint:-
// and are created manually in postMigrate.
type PipelineStepDependencyModel struct {
    StepID      int64 `gorm:"primaryKey;column:step_id;constraint:-"`
    DependsOnID int64 `gorm:"primaryKey;column:depends_on_id;constraint:-"`
}
func (PipelineStepDependencyModel) TableName() string { return "pipeline_step_dependencies" }
```

### postMigrate additions

Add to the `constraints` slice in `postMigrate`:
- `pipeline_step_dependencies.step_id → pipeline_steps(id) ON DELETE CASCADE`
- `pipeline_step_dependencies.depends_on_id → pipeline_steps(id) ON DELETE CASCADE`

Add to `AutoMigrate` call: `&PipelineModel{}`, `&PipelineStepModel{}`, `&PipelineStepDependencyModel{}`

## Mapper and Store Implementation

### PipelineMapper

```go
// infrastructure/persistence/pipeline_mapper.go

type PipelineMapper struct{}

// ToDomain reconstructs the full Pipeline aggregate from a PipelineModel
// that has been preloaded with Steps and their Dependencies.
func (m PipelineMapper) ToDomain(model PipelineModel) pipeline.Pipeline

// ToModel converts a Pipeline to a PipelineModel (without Steps — steps are
// managed separately in PipelineStore.Save to allow reconciliation).
func (m PipelineMapper) ToModel(p pipeline.Pipeline) PipelineModel
```

The mapper builds a kind→id map from loaded StepModels, then for each step reads its dependency rows and reconstructs `ReconstructStep(id, kind, config, dependsOnIDs)`.

### PipelineStore

```go
// infrastructure/persistence/pipeline_store.go

type PipelineStore struct {
    database.Repository[pipeline.Pipeline, PipelineModel]
}

func NewPipelineStore(db database.Database) PipelineStore
```

**Find/FindOne override** (aggregate loading requires Preload):

```go
func (s PipelineStore) FindOne(ctx context.Context, options ...repository.Option) (pipeline.Pipeline, error) {
    // apply conditions, then Preload("Steps.Dependencies")
}

func (s PipelineStore) Find(ctx context.Context, options ...repository.Option) ([]pipeline.Pipeline, error) {
    // same
}
```

**Save logic** (reconcile-on-update pattern):
1. Upsert `PipelineModel` (Create if id==0, Save otherwise)
2. Load existing `PipelineStepModel` rows for this pipeline
3. Delete rows whose IDs are not in the new step set (ON DELETE CASCADE removes their dependency rows)
4. Create/Save each step model
5. Delete all `PipelineStepDependencyModel` rows for steps in this pipeline
6. Insert fresh dependency rows from each step's `DependsOn()` slice
7. Reload the full aggregate via FindOne and return

**Delete**: call `s.DB(ctx).Delete(&PipelineModel{ID: p.ID()})` — cascade handles the rest.

## Key Decisions

- **Aggregate-at-a-time save**: steps and edges are always saved together with the pipeline. No separate step store. This simplifies callers and keeps the aggregate invariant intact.
- **Reconcile via delete-and-reinsert for dependencies**: dependency edges are deleted and re-inserted on every save. Steps themselves are reconciled (add/remove) to preserve step IDs across updates.
- **Composite PK on dependency table uses `constraint:-`** to work around GORM bug #7693 (same pattern as `FileModel.Commit` in this codebase). FKs are created in `postMigrate`.
- **`config` stored as JSONB**: empty map serialises to `{}`. On load, null/empty deserialises to an empty `map[string]any`.
- **Catalog is stateless and constructed once**: `NewCatalog()` builds the static definition map. Presets (`RAGOnly`, `Full`) call `Build` with a fixed kind list.
- **`WithRepoID` re-exported from `repository/query.go`**: no duplication; the pipeline options file is a thin convenience wrapper so callers only import `domain/pipeline`.

## Testing Notes

- Unit tests for `Pipeline` behaviours live in `domain/pipeline/pipeline_test.go`
- Unit tests for `Catalog` live in `domain/pipeline/catalog_test.go`
- Store tests live in `infrastructure/persistence/pipeline_store_test.go` and use `testdb.New(t)` (SQLite in-memory with all migrations applied)
- Store tests must add a `RepositoryModel` row before saving a pipeline (FK constraint)
- Table-driven test format is preferred for `With`/`Without`/`TopologicalOrder`/`Validate`/cycle cases
