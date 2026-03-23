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

### 4. API — `config/chunking` sub-resource (`infrastructure/api/v1/repositories.go`)

Chunk settings are exposed under a `/config/` namespace, establishing a convention for future per-repo config scopes (e.g. `/config/indexing`, `/config/notifications`). Each scope maps to a distinct domain concern and will evolve independently.

Routes:

- `GET  /{id}/config/chunking` — returns current `chunk_size`, `chunk_overlap`, `chunk_min_size` (null means the global default applies).
- `PUT  /{id}/config/chunking` — replaces all three fields atomically. Validate `chunk_overlap < chunk_size` and all provided values are positive; return 400 otherwise. A null field clears the override and reverts to the global default.

DTOs live in `infrastructure/api/v1/dto/`, following the same shape as `TrackingConfigResponse` / `TrackingConfigUpdateRequest`.

The existing `/{id}/tracking-config` routes are unaffected. New config scopes added later follow the same `/{id}/config/{scope}` pattern.

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

- **`/config/{scope}` namespace**: each scope (chunking, and future ones) maps to a distinct domain concern. The URL makes it self-documenting — a consumer looking at `/config/chunking` knows exactly what they're dealing with. Scopes change at different rates and for different reasons, so keeping them separate avoids a sprawling catch-all config endpoint.
- **Nullable pointer fields** over zero-value ints: allows distinguishing "user set 0" (invalid) from "not configured" (use global default).
- **No automatic re-index on settings change**: changing chunk params mid-life would produce mixed-size chunks for the same repo, which is confusing. Users should delete existing index data and re-index.
- **All three chunk params exposed**: `chunk_size`, `chunk_overlap`, and `chunk_min_size` are all configurable per-repo since all three affect indexing behaviour and are already present in `ChunkParams`.

## Patterns Found in Codebase

- All stores embed `database.Repository[D, E]` — adding fields to `RepositoryModel` requires updating only the mapper, not the store.
- GORM AutoMigrate is the only migration mechanism — no SQL files needed.
- DTOs live in `infrastructure/api/v1/dto/` and are separate from domain types.
- Validation happens in the HTTP handler before calling the service layer.
- Existing sub-resource pattern: `/{id}/tracking-config` (GET + PUT) in `infrastructure/api/v1/repositories.go`.
