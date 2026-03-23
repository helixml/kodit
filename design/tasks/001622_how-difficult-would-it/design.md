# Design: Per-Repository RAG Chunk Settings

## Difficulty Assessment

**Medium difficulty** — roughly 1–2 days. The codebase is clean and well-layered; the chunk params infrastructure already exists. The work is additive and follows established patterns throughout.

## Current Architecture

- `infrastructure/chunking/chunks.go`: `ChunkParams{Size, Overlap, MinSize}` — defaults 1500/200/50.
- `internal/config/env.go`: Global env vars `CHUNK_SIZE`, `CHUNK_OVERLAP`, `CHUNK_MIN_SIZE` parsed at startup.
- `cmd/kodit/serve.go`: Builds a single global `ChunkParams` from config and passes it to `kodit.WithChunkParams(...)`.
- `kodit.go` / `handlers.go`: The global `ChunkParams` is injected into the `ChunkFiles` handler once at startup and applied to every repository uniformly.
- `infrastructure/persistence/models.go` (`RepositoryModel`): No chunk fields — it's a table with URI, paths, scan metadata only.
- `infrastructure/api/v1/repositories.go`: CRUD endpoints for repositories, plus sub-resources like `/{id}/tracking-config` (GET + PUT) for scoped configuration.

## Proposed Changes

### 1. Database model (`infrastructure/persistence/models.go`)

Add nullable columns to `RepositoryModel`. GORM AutoMigrate will add them automatically:

```go
ChunkSize    *int `gorm:"column:chunk_size"`
ChunkOverlap *int `gorm:"column:chunk_overlap"`
ChunkMinSize *int `gorm:"column:chunk_min_size"`
```

### 2. Domain model (`domain/repository/repository.go`)

Add optional chunk settings to the `Repository` domain object. Use pointer fields to distinguish "not set" from zero.

### 3. Mapper (`infrastructure/persistence/mappers.go`)

Update the `RepositoryModel` ↔ `repository.Repository` mapper to include the new fields.

### 4. API — `indexing-config` sub-resource (`infrastructure/api/v1/repositories.go`)

The codebase already has a `/{id}/tracking-config` sub-resource (GET + PUT) for repository-scoped configuration. Chunk settings follow the same pattern:

- `GET /{id}/indexing-config` — returns current `chunk_size`, `chunk_overlap`, `chunk_min_size` (null fields mean the global default applies).
- `PUT /{id}/indexing-config` — replaces all three fields. Validate `chunk_overlap < chunk_size` and all provided values are positive; return 400 otherwise. A null field clears the override and reverts to the global default.

DTOs live in `infrastructure/api/v1/dto/` following the same shape as `TrackingConfigResponse` / `TrackingConfigUpdateRequest`.

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
if repo.ChunkMinSize() != nil {
    params.MinSize = *repo.ChunkMinSize()
}
```

## Key Decisions

- **Nullable pointer fields** over zero-value ints: allows distinguishing "user set 0" (invalid) from "not configured" (use global default).
- **No automatic re-index on settings change**: changing chunk params mid-life would produce mixed-size chunks for the same repo, which is confusing. Users should delete existing index data and re-index.
- **All three chunk params exposed**: `chunk_size`, `chunk_overlap`, and `chunk_min_size` are all configurable per-repo since all three affect indexing behaviour and are already present in `ChunkParams`.
- **Sub-resource over PATCH on root**: chunk settings are exposed at `/{id}/indexing-config` (GET + PUT), matching the existing `/{id}/tracking-config` pattern. This keeps configuration concerns separated from the repository identity fields and is consistent with the rest of the API.

## Patterns Found in Codebase

- All stores embed `database.Repository[D, E]` — adding fields to `RepositoryModel` requires updating only the mapper, not the store.
- GORM AutoMigrate is the only migration mechanism — no SQL files needed.
- DTOs live in `infrastructure/api/v1/dto/` and are separate from domain types.
- Validation happens in the HTTP handler before calling the service layer.
