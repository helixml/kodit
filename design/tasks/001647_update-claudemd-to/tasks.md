# Implementation Tasks

- [x] Add swag annotation block to `GetChunkingConfig` in `infrastructure/api/v1/repositories.go` (GET `/repositories/{id}/config/chunking`, returns `dto.ChunkingConfigResponse`)
- [x] Add swag annotation block to `UpdateChunkingConfig` in `infrastructure/api/v1/repositories.go` (PUT `/repositories/{id}/config/chunking`, body `dto.ChunkingConfigUpdateRequest`, returns `dto.ChunkingConfigResponse`)
- [x] Add swag annotation block to `Grep` in `infrastructure/api/v1/repositories.go` (GET `/repositories/{id}/grep`, deprecated, returns `dto.GrepResponse`, note replacement endpoint in description)
- [~] Add API handler annotation rule to `CLAUDE.md` requiring all chi-registered handlers to have complete swag blocks before merging
- [ ] Run `make build` and confirm it succeeds (swag generation must pass)
