# Design: Folder-Based Repository Creation Mode

## Overview

Kodit's repository pipeline has two git-specific phases that must be bypassed for `file://` URIs:

1. **Clone phase** (`application/handler/repository/clone.go`): calls `cloner.Clone()` which runs `git clone`
2. **Sync/Update phase** (`application/handler/repository/sync.go`): calls `cloner.Update()` which runs `git fetch` + `git pull`

For `file://` URIs, the local path is already the working copy. No git network operations are needed.

## Key Files

| File | Role |
|------|------|
| `infrastructure/git/cloner.go` | `RepositoryCloner` — Clone, Update, ClonePathFromURI |
| `application/handler/repository/clone.go` | Clone task handler |
| `application/handler/repository/sync.go` | Sync task handler (calls `cloner.Update`) |
| `domain/repository/repository.go` | `NewRepository(url)` — may validate URI |

## Approach

Add a `isFileURI(uri string) bool` helper (e.g. `strings.HasPrefix(uri, "file://")`) and a `localPathFromFileURI(uri string) string` helper that strips the `file://` prefix.

### 1. `RepositoryCloner.Clone()` — skip git clone for file URIs

```go
func (c *RepositoryCloner) Clone(ctx context.Context, remoteURI string) (string, error) {
    if isFileURI(remoteURI) {
        return localPathFromFileURI(remoteURI), nil
    }
    // existing git clone logic ...
}
```

### 2. `RepositoryCloner.ClonePathFromURI()` — return local path directly for file URIs

```go
func (c *RepositoryCloner) ClonePathFromURI(uri string) string {
    if isFileURI(uri) {
        return localPathFromFileURI(uri)
    }
    return filepath.Join(c.cloneDir, sanitizeURIForPath(uri))
}
```

### 3. `RepositoryCloner.Update()` — skip fetch/pull for file URIs

In `Update()`, after checking the path exists and before calling `updateBranch`/`updateTag`, check if the repo is a file URI and return early without pulling:

```go
if isFileURI(repo.RemoteURL()) {
    return clonePath, nil  // local folder; no git pull needed
}
```

### 4. URI helper

```go
func isFileURI(uri string) bool {
    return strings.HasPrefix(uri, "file://")
}

func localPathFromFileURI(uri string) string {
    return strings.TrimPrefix(uri, "file://")
}
```

`file:///home/user/project` → `/home/user/project` (three slashes: `file://` stripped, leaving `/home/...`). This is correct standard behaviour for `file://` URIs on Linux.

## Learnings from Codebase

- `sanitizeURIForPath` already strips `file____`/`file___` prefixes for path sanitization — but this function is only used for git repos now, no change needed there.
- The `ClonePathFromURI` return value becomes the `cloned_path` stored in `git_repos.ClonedPath`. For file URIs we want this to be the actual local path, not a subdirectory of `cloneDir`.
- `RepositoryCloner.Update()` has a relocation fallback (if stored path is stale, re-clones). For file URIs we must **not** try to re-clone if the path is gone — the directory is the user's responsibility. The early return before the `os.Stat` check handles this.
- Scanning (`ScanAllBranches`, commit scanning) uses standard git commands on the local path — these continue to work for local git repos unchanged.
- No changes to domain model, persistence, or API layer are required. The URI is stored as-is in `RemoteURI`.
- The `prescribedOps.CreateNewRepository()` pipeline (Clone → Sync) still runs in full; only the git network calls inside each step are skipped.
