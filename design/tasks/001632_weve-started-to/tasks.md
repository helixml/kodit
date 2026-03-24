# Implementation Tasks

## Domain — new `domain/pipeline` package

- [ ] Create `domain/pipeline/pipeline_step.go`: `StepKind` type, `PipelineStep` struct, `NewBuiltInStep` constructor, `ReconstructStep` constructor, getters
- [ ] Create `domain/pipeline/pipeline.go`: `Pipeline` struct, `NewPipeline` / `ReconstructPipeline` constructors, `Steps`, `Operations`, `HasOperation`, `Validate` methods, `builtInDependencies` map, `coreOperations` list
- [ ] Create preset factory functions in `domain/pipeline/presets.go`: `DefaultPipeline`, `RAGOnlyPipeline`, `FullPipeline` — each builds the step list from `builtInDependencies` and assigns positions and `dependsOn` IDs
- [ ] Create `domain/pipeline/store.go`: `Store` interface with `FindByRepo`, `Save`, `Delete`

## Persistence

- [ ] Add `PipelineModel` and `PipelineStepModel` to `infrastructure/persistence/models.go`
- [ ] Add both models to `AutoMigrate` in `infrastructure/persistence/db.go`
- [ ] Create `infrastructure/persistence/pipeline_store.go` implementing `pipeline.Store`: `FindByRepo` loads pipeline + steps and maps to domain; `Save` upserts pipeline then replaces steps; `Delete` removes pipeline (cascade deletes steps)
- [ ] Wire `PipelineStore` into `kodit.Client` in `kodit.go`

## Application — queue integration

- [ ] Add `PipelinePreset string` field to `RepositoryAddParams` in `application/service/repository.go`
- [ ] In `RepositoryService.Add`: after saving the repository, resolve the preset to a `Pipeline` using the factory function, save it via `PipelineStore`, then enqueue tasks — in that order
- [ ] In every place that calls `prescribed.ScanAndIndexCommit()` / `IndexCommit()` / `RescanCommit()` and enqueues the result, load the pipeline via `PipelineStore.FindByRepo` and intersect the candidate list with `pipeline.Operations()`
- [ ] Add lazy migration helper: if `FindByRepo` returns `ErrNotFound`, create and save a `DefaultPipeline` before proceeding

## API

- [ ] Add `pipeline_preset` field to `RepositoryCreateAttributes` DTO in `infrastructure/api/v1/dto/repository.go`
- [ ] Add `GetPipeline` handler — load pipeline by repo ID, return full pipeline with steps as JSON
- [ ] Add `ReplaceSteps` handler — parse new step list, call `Pipeline.Validate()`, save; return 400 with error on invalid input
- [ ] Add `RemoveStep` handler — find step by ID, check no other steps depend on it, remove and save; return 400 if step has dependents
- [ ] Register new routes: `GET /{id}/pipeline`, `PUT /{id}/pipeline/steps`, `DELETE /{id}/pipeline/steps/{step_id}`

## Tests

- [ ] Unit tests for `Pipeline.Validate()`: missing dependency, removed core step, cycle detection, valid config
- [ ] Unit tests for `Pipeline.Operations()`: returns correct ordered list for each preset
- [ ] Unit tests for preset factories: step sets and dependency IDs are consistent
- [ ] Unit tests for `PipelineStore`: `FindByRepo` round-trips steps and `dependsOn` correctly
- [ ] Integration/handler tests for the three new pipeline endpoints (happy path + validation errors)
- [ ] Test that `POST /repositories` with `pipeline_preset` persists the pipeline before any task is enqueued
