# Requirements

## Problem

Three endpoints in `infrastructure/api/v1/repositories.go` are registered in the router but lack swag annotations, so they are invisible in the generated OpenAPI spec:

| Handler | Method | Path |
|---|---|---|
| `GetChunkingConfig` | GET | `/repositories/{id}/config/chunking` |
| `UpdateChunkingConfig` | PUT | `/repositories/{id}/config/chunking` |
| `Grep` | GET | `/repositories/{id}/grep` (deprecated) |

Additionally, CLAUDE.md contains no rule requiring swag annotations, so new endpoints can be added without them going unnoticed.

## User Stories

**As an API consumer**, I want all exposed endpoints to appear in the OpenAPI docs so I can discover and use them without reading source code.

**As a developer**, I want CLAUDE.md to remind me to add swag annotations when creating API handlers so the spec stays complete.

## Acceptance Criteria

- `GetChunkingConfig`, `UpdateChunkingConfig`, and `Grep` all have complete swag comment blocks (Summary, Description, Tags, Param, Success, Failure, Security, Router).
- `Grep` is marked `@Deprecated` and its description notes it is superseded by `GET /api/v1/search/grep`.
- CLAUDE.md has a section (or addition to an existing section) stating that every handler registered in a chi router **must** have a full swag annotation block before merging.
- `make build` (which runs `swag init`) continues to succeed after the changes.
