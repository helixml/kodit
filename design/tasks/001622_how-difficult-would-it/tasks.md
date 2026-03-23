# Implementation Tasks

- [ ] Add nullable `ChunkSize *int` and `ChunkOverlap *int` fields to `RepositoryModel` in `infrastructure/persistence/models.go` (AutoMigrate handles the schema change)
- [ ] Add optional chunk fields to the `Repository` domain type in `domain/repository/repository.go` with getter methods
- [ ] Update `RepositoryModel` ↔ `Repository` mapper in `infrastructure/persistence/mappers.go` to map the new fields
- [ ] Add `PATCH /{id}` route to `RepositoriesRouter` in `infrastructure/api/v1/repositories.go` accepting `chunk_size` and `chunk_overlap`; validate `overlap < size` and both positive; return 400 on invalid input
- [ ] Update repository GET and List DTOs in `infrastructure/api/v1/dto/` to include `chunk_size` and `chunk_overlap` (nullable)
- [ ] In `ChunkFiles.Execute()` (`application/handler/indexing/chunk_files.go`), override global `h.params` with per-repo values when set on the repository
- [ ] Add unit tests for the updated `ChunkFiles.Execute()` covering: no per-repo settings (uses global), only `chunk_size` set, both fields set
- [ ] Add integration/API test for the PATCH endpoint covering valid update, invalid overlap >= size, and missing fields (should be no-op)
