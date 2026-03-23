# Requirements: Per-Repository RAG Chunk Settings

## User Stories

**As a developer**, I want to set custom `chunk_size`, `chunk_overlap`, and `chunk_min_size` values on a specific repository so that I can tune retrieval quality for that codebase without affecting other repositories.

**As a developer**, I want repository-level chunk settings to override the global server defaults so that I can experiment per-repo without restarting the server.

## Acceptance Criteria

1. A repository exposes an `indexing-config` sub-resource (`GET /repositories/{id}/indexing-config`, `PUT /repositories/{id}/indexing-config`) for reading and updating `chunk_size`, `chunk_overlap`, and `chunk_min_size` (all optional integers).
2. When set, those values are used during the next indexing run for that repository instead of the global defaults.
3. When not set (null), the global server defaults (`CHUNK_SIZE`, `CHUNK_OVERLAP`, `CHUNK_MIN_SIZE`) apply.
4. `GET /repositories/{id}/indexing-config` returns the currently configured `chunk_size`, `chunk_overlap`, and `chunk_min_size` (null if the global default applies).
5. Setting values out of range (e.g. `chunk_overlap >= chunk_size`, or any value ≤ 0) returns a 400 error.
6. Changing chunk settings on an already-indexed repository does NOT automatically re-index; the user must trigger a re-index manually.
