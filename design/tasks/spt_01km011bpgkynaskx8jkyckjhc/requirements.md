# Requirements: Folder-Based Repository Creation Mode

## User Stories

**As a user, I want to pass a `file:///path/to/dir` URI when adding a repository so that Kodit indexes a local directory without attempting git operations.**

### Acceptance Criteria

1. When a repository is added with a URI beginning with `file://`, Kodit:
   - Accepts the URI without error
   - Does **not** perform a git clone (the directory already exists locally)
   - Sets the `cloned_path` in the database to the local filesystem path extracted from the URI (e.g. `file:///home/user/project` → `/home/user/project`)

2. When syncing a `file://` repository, Kodit:
   - Skips `git fetch` and `git pull` operations
   - Still scans branches/commits if the directory is a git repo
   - Proceeds to the indexing pipeline as normal

3. A `file://` repository behaves identically to a cloned git repository in all downstream steps (commit scanning, enrichment, MCP search).

4. Non-`file://` URIs continue to work exactly as before — no regression.
