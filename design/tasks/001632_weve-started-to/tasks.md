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

- [ ] Add `GetPipelineConfig` handler — load repo, return `{"steps": [...]}` JSON
- [ ] Add `UpdatePipelineConfig` handler — parse `{"steps": [...]}`, call `ReconstructPipelineConfig`, call `Validate`, save repo
- [ ] Add `InitPipelineConfig` handler — parse `{"preset": "..."}`, build config via the appropriate constructor, validate `full` requires text provider, save repo
- [ ] Register three new routes in `infrastructure/api/api_server.go` (or `repositories.go` route group): `GET`, `PUT`, `POST /init` under `/{id}/config/pipeline`

## Tests

- [ ] Unit tests for `PipelineConfig.Validate()`: missing dependency, removed core step, unknown operation, valid config
- [ ] Unit tests for `PipelineConfig.Filter()`: returns intersection in correct order
- [ ] Unit tests for preset constructors: verify step sets match expected operations
- [ ] Integration/handler tests for the three new API endpoints (happy path + validation errors)
