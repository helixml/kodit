# Requirements: Folder-Based Repository Creation Mode

## Development Approach

Use TDD. Write a single high-level application service e2e test first (in `application/service/`), covering the full flow from `Repository.Add()` through clone/update to the working copy path being set correctly. Make it fail, then implement until it passes.

## User Stories

**As a developer**, I want to index a local directory by passing a `file://` URI so that Kodit can index code that isn't in a remote git repository.

**As a developer**, I want local directory indexing to skip git operations (clone, pull, fetch) so that indexing works on plain folders without git history.

**As a developer**, I want the local directory path to be used directly as the working copy so that Kodit reads files from the original location without copying them.

## Acceptance Criteria

1. `POST /api/v1/repositories` with `remote_uri: "file:///path/to/folder"` creates a repository entry without error.
2. The cloned path stored in the database equals the local filesystem path (e.g. `/path/to/folder`), not a new directory under the clone root.
3. No git operations (clone, pull, fetch, checkout) are attempted for `file://` URIs.
4. Indexing (snippet extraction, embeddings, etc.) proceeds normally after the "clone" step is skipped.
5. Syncing a `file://` repository re-scans the directory without attempting git pull.
6. The `sanitized_remote_uri` deduplication still works for `file://` URIs (same path = same repo).
7. An invalid local path (directory does not exist) returns a meaningful error.
