# Design

## Swag Annotation Pattern

This codebase uses [swaggo/swag](https://github.com/swaggo/swag). Annotations live in the Go doc comment immediately above the handler function, using tab-indented `//` lines. The canonical style seen in this file:

```go
// GetChunkingConfig handles GET /api/v1/repositories/{id}/config/chunking.
//
//	@Summary		Get chunking config
//	@Description	Get current chunking configuration for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Repository ID"
//	@Success		200	{object}	dto.ChunkingConfigResponse
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/config/chunking [get]
```

Key observations from existing annotated handlers:
- `@Tags` is always `repositories` for handlers in this file.
- `@Security APIKeyAuth` is required on all non-deprecated endpoints.
- DELETE/POST with no body still include `@Accept json`.
- Deprecated endpoints use `@Deprecated` and omit `@Security`.
- The `@Router` path matches the chi `router.Route` mount point, not the full URL (swag prepends `/api/v1` via the main annotation).

## Endpoints to Annotate

### GetChunkingConfig — `GET /repositories/{id}/config/chunking`
- Returns `dto.ChunkingConfigResponse`
- Errors: 404 (repo not found), 500

### UpdateChunkingConfig — `PUT /repositories/{id}/config/chunking`
- Body: `dto.ChunkingConfigUpdateRequest`
- Returns `dto.ChunkingConfigResponse`
- Errors: 400 (bad body), 404, 500

### Grep — `GET /repositories/{id}/grep` (deprecated)
- Returns `dto.GrepResponse`
- Mark `@Deprecated`
- Description must say "Deprecated: use GET /api/v1/search/grep instead."
- No `@Security` (deprecated endpoints in this codebase omit it, see `ListCommitEmbeddingsDeprecated`)

## CLAUDE.md Addition

Add a new rule under the existing **Go** section (or a new **API** section):

```
## API Handlers

Every handler function registered in a chi router **must** have a complete swag annotation
block before merging. Required fields: @Summary, @Description, @Tags, @Param (all path/query/body
params), @Success, @Failure (at least 404 and 500), @Security (for non-deprecated endpoints),
and @Router. Deprecated endpoints must include @Deprecated and note the replacement in @Description.
```

## Files Changed

- `infrastructure/api/v1/repositories.go` — add swag comments to 3 handlers
- `CLAUDE.md` — add API handler annotation rule
