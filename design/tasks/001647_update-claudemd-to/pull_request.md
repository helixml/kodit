# Add swag annotations to missing repository endpoints

## Summary
Three endpoints registered in `infrastructure/api/v1/repositories.go` were missing swag annotations, making them invisible in the generated OpenAPI spec. This PR adds full annotations and updates CLAUDE.md to prevent future omissions.

## Changes
- Added swag annotation block to `GetChunkingConfig` (`GET /repositories/{id}/config/chunking`)
- Added swag annotation block to `UpdateChunkingConfig` (`PUT /repositories/{id}/config/chunking`)
- Added swag annotation block to `Grep` (`GET /repositories/{id}/grep`, marked `@Deprecated` with note to use `/api/v1/search/grep`)
- Added "API Handlers" rule to `CLAUDE.md` requiring complete swag blocks on all chi-registered handlers
- Regenerated `docs/swagger/` (swagger.json, swagger.yaml, docs.go)

## Testing
Ran `swag init` directly — generation succeeds with all three new endpoints appearing in the spec.
