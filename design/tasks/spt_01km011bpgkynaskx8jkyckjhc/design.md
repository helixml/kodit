# Design: Folder-Based Repository Creation Mode

## Overview

Kodit's repository pipeline has two git-specific phases to consider for `file://` URIs:

1. **Clone phase** (`application/handler/repository/clone.go`): calls `cloner.Clone()` which runs `git clone` — always skipped for `file://` (path already exists)
2. **Sync/Update phase** (`application/handler/repository/sync.go`): calls `cloner.Update()` which runs `git fetch` + `git pull` — only skipped if the local directory is **not** a git repo

A local directory pointed to by a `file://` URI may or may not be a git repository. Both cases must be handled:
- **Git recognises it** (`git rev-parse --git-dir` exits 0): skip clone, but still run fetch/pull and scan branches/commits normally
- **Git does not recognise it**: skip clone, skip all git operations; index files as-is

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

### 3. `RepositoryCloner.Update()` — skip fetch/pull only for non-git file URIs

In `Update()`, after the path exists check and before calling `updateBranch`/`updateTag`, detect whether the local path is a git repo. If it is a git repo, proceed with fetch/pull as normal. If not, return early:

```go
if isFileURI(repo.RemoteURL()) && !isGitRepo(clonePath) {
    return clonePath, nil  // plain local folder; skip git operations
}
```

Add a helper `isGitRepo(path string) bool` that asks git itself, so it handles regular repos, bare repos, and worktrees correctly:

```go
func isGitRepo(path string) bool {
    cmd := exec.Command("git", "rev-parse", "--git-dir")
    cmd.Dir = path
    return cmd.Run() == nil
}
```

Also, the relocation fallback in `Update()` (re-clone if stored path is stale) must not trigger for file URIs regardless of git status — the directory is the user's responsibility. Guard that block with `!isFileURI(repo.RemoteURL())`.

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
