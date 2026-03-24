# Implementation Tasks

## Domain

- [ ] Create `domain/repository/pipeline_config.go` with `PipelineConfig` struct, `stepDependencies` map, `coreSteps` list, constructors (`DefaultPipelineConfig`, `RAGOnlyPipelineConfig`, `FullPipelineConfig`, `ReconstructPipelineConfig`), and methods (`Steps`, `Filter`, `Contains`, `Validate`)
- [ ] Add `pipelineConfig PipelineConfig` field to `Repository` in `domain/repository/repository.go`; add `PipelineConfig()` getter and `WithPipelineConfig()` mutator; update `NewRepository` to initialise with `DefaultPipelineConfig`; update `ReconstructRepository` signature

## Persistence

- [ ] Add `PipelineSteps string` column to `RepositoryModel` in `infrastructure/persistence/models.go` (GORM AutoMigrate handles the column; default `'[]'`)
- [ ] Update `RepositoryMapper` to accept `hasTextProvider bool`; decode JSON steps in `ToDomain` (empty → server default); encode in `ToModel`
- [ ] Thread `hasTextProvider` through the store constructor so the mapper receives it

## Application — queue integration

- [ ] In every place that calls `prescribed.ScanAndIndexCommit()` / `IndexCommit()` / `RescanCommit()` and enqueues the result, load the repository and wrap the call with `repo.PipelineConfig().Filter(prescribed.XXX())`

## API

- [ ] Add `pipeline_preset` field (`string`, optional) to `RepositoryCreateAttributes` DTO (`infrastructure/api/v1/dto/repository.go`) and `RepositoryAddParams` (`application/service/repository.go`)
- [ ] In `RepositoryService.Add`, resolve the preset to a `PipelineConfig` (error on unknown/unsupported preset) and call `WithPipelineConfig` on the repository before the first `EnqueueOperations` call
- [ ] Add `GetPipelineConfig` handler — load repo, return `{"steps": [...]}` JSON
- [ ] Add `UpdatePipelineConfig` handler — parse `{"steps": [...]}`, call `ReconstructPipelineConfig`, call `Validate`, save repo
- [ ] Register two new routes in the `/{id}/config` route group: `GET /pipeline` and `PUT /pipeline`

## Tests

- [ ] Unit tests for `PipelineConfig.Validate()`: missing dependency, removed core step, unknown operation, valid config
- [ ] Unit tests for `PipelineConfig.Filter()`: returns intersection in correct order
- [ ] Unit tests for preset constructors: verify step sets match expected operations
- [ ] Integration/handler tests for the two new pipeline config endpoints (happy path + validation errors)
- [ ] Test that `POST /repositories` with `pipeline_preset` persists the config before any task is enqueued
