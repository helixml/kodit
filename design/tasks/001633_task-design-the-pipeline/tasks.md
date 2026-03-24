# Implementation Tasks

## Domain

- [ ] Create `domain/pipeline/pipeline.go` — `Step` entity (fields, `NewStep`, `ReconstructStep`, getters)
- [ ] Add `Pipeline` aggregate to `domain/pipeline/pipeline.go` (`NewPipeline`, `ReconstructPipeline`, getters)
- [ ] Implement `Pipeline.Has`, `Step(kind)`, `Roots`, `DependentsOf`
- [ ] Implement `Pipeline.TopologicalOrder` (Kahn's algorithm)
- [ ] Implement `Pipeline.Validate` (cycle, dangling dependsOn, duplicate kinds)
- [ ] Implement `Pipeline.With` (returns new pipeline; errors if dependsOn ID missing)
- [ ] Implement `Pipeline.Without` (BFS transitive dependent removal)
- [ ] Create `domain/pipeline/catalog.go` — `Catalog` with static step definitions for all commit-level operations
- [ ] Implement `Catalog.Build(repoID, kinds)` with auto-wired dependsOn edges
- [ ] Implement `Catalog.RAGOnly` and `Catalog.Full` presets
- [ ] Create `domain/pipeline/store.go` — `Store` interface (`Find`, `FindOne`, `Save`, `Delete`)
- [ ] Create `domain/pipeline/options.go` — `WithRepoID` re-export and `WithPipelineID`

## Persistence — Models

- [ ] Create `infrastructure/persistence/pipeline_models.go` — `PipelineModel`, `PipelineStepModel`, `PipelineStepDependencyModel` with GORM tags (including `constraint:-` on dependency FK columns)

## Persistence — Schema

- [ ] Add `&PipelineModel{}`, `&PipelineStepModel{}`, `&PipelineStepDependencyModel{}` to `AutoMigrate` in `infrastructure/persistence/db.go`
- [ ] Add FK constraints for `pipeline_step_dependencies` to the `postMigrate` `constraints` slice in `infrastructure/persistence/db.go`

## Persistence — Mapper and Store

- [ ] Create `infrastructure/persistence/pipeline_mapper.go` — `PipelineMapper` (`ToDomain` reconstructs full aggregate; `ToModel` for pipeline header only)
- [ ] Create `infrastructure/persistence/pipeline_store.go` — `PipelineStore` embedding `database.Repository[pipeline.Pipeline, PipelineModel]`
- [ ] Implement `PipelineStore.FindOne` and `Find` with `Preload("Steps.Dependencies")`
- [ ] Implement `PipelineStore.Save` with reconcile logic (upsert pipeline, reconcile steps, delete-and-reinsert dependency rows)
- [ ] Implement `PipelineStore.Delete`

## Tests

- [ ] `domain/pipeline/pipeline_test.go` — table-driven tests for `With`, `Without`, `TopologicalOrder`, `Validate`, cycle detection
- [ ] `domain/pipeline/catalog_test.go` — `Build`, `RAGOnly`, `Full`, dependency auto-wiring, missing-dependency error
- [ ] `infrastructure/persistence/pipeline_store_test.go` — save, load, round-trip equality, cascade delete via repo deletion
