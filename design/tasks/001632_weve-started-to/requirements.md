# Requirements: Per-Repository Pipeline Configuration

## Context

Kodit already has repository-level chunking config stored in `git_repos`. Processing pipelines are currently defined globally via `PrescribedOperations` (a server-side struct with `enrichments`/`examples` booleans). This is too coarse: different repositories may need different subsets of the available pipeline steps.

## User Stories

### US1 — Persist a pipeline per repository
As a developer, I want each repository to have its own pipeline configuration stored in the database so that I can run different processing steps per repository without restarting the server.

**Acceptance criteria:**
- A repository's pipeline config is stored in `git_repos` and survives restarts.
- A newly created repository receives a default pipeline config based on server capabilities (text provider present → full enrichments; absent → RAG only).
- When a commit is enqueued, the operations used come from the repository's own pipeline config, not a global preset.

### US2 — Edit pipeline steps via API
As a Go API user, I want to PUT a new set of pipeline steps for a repository so that I can disable steps I don't need.

**Acceptance criteria:**
- `GET /api/v1/repositories/{id}/config/pipeline` returns the current enabled steps.
- `PUT /api/v1/repositories/{id}/config/pipeline` with `{"steps": [...]}` replaces the pipeline.
- The API rejects unknown operation names (400).
- The API rejects configurations where a step's required dependency is absent (400, with an error message naming the missing dependency).
- Infrastructure steps (clone, sync, scan) cannot be removed (400 if omitted).

### US3 — Initialise with a preset
As a Go API user, I want to POST a preset name to reset a repository's pipeline so that I can quickly switch to RAG-only or full-enrichment mode.

**Acceptance criteria:**
- `POST /api/v1/repositories/{id}/config/pipeline/init` with `{"preset": "rag-only" | "full" | "default"}` replaces the pipeline with the corresponding preset.
- `rag-only`: clone, sync, scan, extract-snippets, BM25, code-embeddings.
- `full`: all operations (requires text provider; returns 400 if none configured).
- `default`: same as server startup default (enrichments iff text provider present).
