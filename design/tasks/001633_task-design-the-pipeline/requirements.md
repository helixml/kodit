# Requirements: Pipeline Domain — Entities, Store, and Database Schema

## Overview

Add a per-repository processing pipeline domain to kodit. A pipeline is an ordered, dependency-aware set of processing steps (e.g. extract snippets, BM25 index, code embeddings, wiki) that determines what gets run for a repository and in what order. Pipelines are modeled as a DAG of steps.

## User Stories

**US-1 — Pipeline as first-class domain entity**
As a developer, I can create a `Pipeline` aggregate that owns a set of `Step` entities with explicit dependency edges, so that the system knows which processing steps to run for a repository and in what order.

**US-2 — Catalog-driven pipeline construction**
As a developer, I can use a `Catalog` to build a `Pipeline` from a list of step kinds (e.g. RAGOnly, Full) and have dependency edges auto-wired, so that pipelines are constructed consistently without manually specifying every edge.

**US-3 — Pipeline persistence**
As a developer, I can save and load a full `Pipeline` aggregate (including its steps and dependency edges) via a `PipelineStore`, so that pipeline configuration survives restarts.

## Acceptance Criteria

### Domain — Step
- `NewStep(kind string, config map[string]any) Step` creates a step with no ID
- `ReconstructStep(id, kind, config, dependsOn)` reconstructs from persistence
- `Step.DependsOn()` returns a copy of the dependency ID slice
- `config` may be nil/empty; getters return an empty map, not nil

### Domain — Pipeline
- `NewPipeline(repoID int64) Pipeline` creates an empty pipeline
- `ReconstructPipeline(id, repoID, steps, createdAt, updatedAt)` reconstructs from persistence
- `Pipeline.Has(kind)` returns true iff a step of that kind exists
- `Pipeline.Step(kind)` returns the step and true, or zero value and false
- `Pipeline.Roots()` returns steps with an empty dependsOn
- `Pipeline.DependentsOf(stepID)` returns steps whose dependsOn includes stepID
- `Pipeline.TopologicalOrder()` returns all steps with dependencies before dependents (Kahn's algorithm)
- `Pipeline.Validate()` returns an error if: the graph contains a cycle, any dependsOn ID does not refer to a step in this pipeline, or duplicate step kinds exist
- `Pipeline.With(step)` returns a new pipeline with the step appended; errors if any dependsOn ID is not present in the pipeline
- `Pipeline.Without(stepID)` returns a new pipeline with the step removed and all transitive dependents also removed
- All mutations return a new Pipeline (immutable); updatedAt is bumped

### Domain — Catalog
- Catalog holds a static map of kind → `{ requires []string }` for every commit-level operation in `domain/task/operation.go`; structural ops (clone, sync, scan, rescan) are excluded
- `Catalog.Build(repoID, kinds)` creates a Pipeline using only the requested kinds; dependsOn edges auto-wired from catalog requires; errors if a required kind is not in the requested set
- `Catalog.RAGOnly(repoID)` builds: `extract_snippets`, `bm25_index`, `code_embeddings`
- `Catalog.Full(repoID)` builds: all known commit-level step kinds

### Database Schema
- Three tables managed by GORM AutoMigrate: `pipelines`, `pipeline_steps`, `pipeline_step_dependencies`
- `pipelines.repo_id` has a UNIQUE constraint and ON DELETE CASCADE FK to `git_repos`
- `pipeline_steps.pipeline_id` has ON DELETE CASCADE FK to `pipelines`
- `pipeline_step_dependencies` both FKs cascade; composite PK `(step_id, depends_on_id)`

### Store
- `PipelineStore.Save(ctx, Pipeline)` upserts the pipeline, its steps, and edges (reconciles: add new, remove missing)
- `PipelineStore.Delete(ctx, Pipeline)` removes the pipeline (cascade handles steps and edges)
- `PipelineStore.FindOne(ctx, ...Option)` loads a pipeline with all steps and edges hydrated
- `PipelineStore.Find(ctx, ...Option)` loads multiple pipelines fully hydrated
- `WithRepoID(id)` option filters by `repo_id` (reuse existing from `repository/query.go`)
- `WithPipelineID(id)` option filters by pipeline ID

### Testing
- Table-driven unit tests for `Pipeline.With`, `Without`, `TopologicalOrder`, `Validate`, cycle detection
- Tests for `Catalog.Build`, `RAGOnly`, `Full`, missing-dependency errors
- Store integration tests using `testdb.New(t)`: save, load, round-trip equality, cascade delete via repo deletion
