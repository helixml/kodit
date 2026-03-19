# Implementation Tasks

## Implementation notes (discovered during implementation)

The primary project is the kodit library (`github.com/helixml/kodit`), not helix.
The original spec referenced helix paths — those tasks are wrong. The actual work is:
- Commit 417f16b (file:// URI support) is already the latest commit — update task is done.
- Add a new `RAG` service to the kodit library that wraps `Repositories.Add` + `Search.Query`
  in conversion/index/search interfaces so callers (e.g. helix) have a clean RAG-oriented API.
- Callers receive the repo ID from `Convert()` and store it themselves.

- [x] Update kodit to commit `417f16b` — already the latest commit, nothing to do
- [x] Add `RAGMode` type and `RAG_MODE` env var to `internal/config/env.go`
- [x] Create `application/service/rag.go` with:
  - `Conversion` interface: `Convert(ctx, localPath string) (int64, error)` — returns kodit repo ID
  - `Indexer` interface: `Index(ctx, repoID int64) error` — no-op
  - `RAG` struct implementing both, backed by `*Repository` and `*Search` services
- [x] Expose `RAG *service.RAG` on `kodit.Client` and wire it in `kodit.go`
- [x] Write tests for `RAG.Convert` and `RAG.Index`
