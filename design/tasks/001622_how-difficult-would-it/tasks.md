# Implementation Tasks

- [ ] Add nullable `ChunkSize *int`, `ChunkOverlap *int`, and `ChunkMinSize *int` fields to `RepositoryModel` in `infrastructure/persistence/models.go` (AutoMigrate handles the schema change)
- [ ] Add optional chunk fields to the `Repository` domain type in `domain/repository/repository.go` with getter methods
- [ ] Update `RepositoryModel` ↔ `Repository` mapper in `infrastructure/persistence/mappers.go` to map the new fields
- [ ] Add `GET /{id}/config/chunking` and `PUT /{id}/config/chunking` routes to `RepositoriesRouter` in `infrastructure/api/v1/repositories.go`; validate `overlap < size` and all values positive; return 400 on invalid input
- [ ] Add `ChunkingConfigResponse` and `ChunkingConfigUpdateRequest` DTOs in `infrastructure/api/v1/dto/` (nullable `chunk_size`, `chunk_overlap`, `chunk_min_size`)
- [ ] In `ChunkFiles.Execute()` (`application/handler/indexing/chunk_files.go`), override global `h.params` with per-repo values when set on the repository
- [ ] Add unit tests for the updated `ChunkFiles.Execute()` covering: no per-repo settings (uses global), only `chunk_size` set, all fields set
- [ ] Add integration/API tests for the new endpoints covering valid update, invalid overlap >= size, null fields clearing overrides, and GET reflecting current state
