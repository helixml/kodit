# Design: Per-Repository RAG Chunk Settings

## Difficulty Assessment

**Medium difficulty** — roughly 1–2 days. The codebase is clean and well-layered; the chunk params infrastructure already exists. The work is additive and follows established patterns throughout.

## Current Architecture

- `infrastructure/chunking/chunks.go`: `ChunkParams{Size, Overlap, MinSize}` — defaults 1500/200/50.
- `internal/config/env.go`: Global env vars `CHUNK_SIZE`, `CHUNK_OVERLAP`, `CHUNK_MIN_SIZE` parsed at startup.
- `cmd/kodit/serve.go`: Builds a single global `ChunkParams` from config and passes it to `kodit.WithChunkParams(...)`.
- `kodit.go` / `handlers.go`: The global `ChunkParams` is injected into the `ChunkFiles` handler once at startup and applied to every repository uniformly.
- `infrastructure/persistence/models.go` (`RepositoryModel`): No chunk fields — it's a table with URI, paths, scan metadata only.
- `infrastructure/api/v1/repositories.go`: CRUD endpoints for repositories (no update/PATCH endpoint currently).

## Proposed Changes

### 1. Database model (`infrastructure/persistence/models.go`)

Add nullable columns to `RepositoryModel`. GORM AutoMigrate will add them automatically:

```go
ChunkSize    *int `gorm:"column:chunk_size"`
ChunkOverlap *int `gorm:"column:chunk_overlap"`
```

### 2. Domain model (`domain/repository/repository.go`)

Add optional chunk settings to the `Repository` domain object. Use pointer fields to distinguish "not set" from zero.

### 3. Mapper (`infrastructure/persistence/mappers.go`)

Update the `RepositoryModel` ↔ `repository.Repository` mapper to include the new fields.

### 4. API — PATCH endpoint (`infrastructure/api/v1/repositories.go`)

Add `PATCH /{id}` route that accepts `chunk_size` and `chunk_overlap` as optional JSON fields. Validate that `chunk_overlap < chunk_size` and both are positive before saving.

Update the repository GET/List DTOs to include `chunk_size` and `chunk_overlap` (null when not set).

### 5. Indexing handler (`application/handler/indexing/chunk_files.go`)

In `ChunkFiles.Execute()`, after fetching the repository, override `h.params` with per-repo values if present:

```go
params := h.params // global default
if repo.ChunkSize() != nil {
    params.Size = *repo.ChunkSize()
}
if repo.ChunkOverlap() != nil {
    params.Overlap = *repo.ChunkOverlap()
}
```

## Key Decisions

- **Nullable pointer fields** over zero-value ints: allows distinguishing "user set 0" (invalid) from "not configured" (use global default).
- **No automatic re-index on settings change**: changing chunk params mid-life would produce mixed-size chunks for the same repo, which is confusing. Users should delete existing index data and re-index.
- **No `chunk_min_size` exposed for now**: it's an edge-case tuning parameter; exposing only `chunk_size` and `chunk_overlap` matches the user request and keeps the surface small.
- **PATCH not PUT**: allows partial updates without requiring the full repository object.

## Patterns Found in Codebase

- All stores embed `database.Repository[D, E]` — adding fields to `RepositoryModel` requires updating only the mapper, not the store.
- GORM AutoMigrate is the only migration mechanism — no SQL files needed.
- DTOs live in `infrastructure/api/v1/dto/` and are separate from domain types.
- Validation happens in the HTTP handler before calling the service layer.
