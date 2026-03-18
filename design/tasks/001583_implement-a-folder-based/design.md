# Design: Folder-Based Repository Creation Mode

## Key Files

| File | Role |
|------|------|
| `infrastructure/git/cloner.go` | `RepositoryCloner` — Clone, Update, Ensure methods |
| `infrastructure/git/adapter.go` | `Adapter` interface — git operations |
| `domain/service/cloner.go` | `Cloner` interface |
| `application/service/repository.go` | `Repository.Add()` — creates repo and enqueues tasks |
| `domain/repository/repository.go` | Domain `Repository` model, `WorkingCopy` |
| `infrastructure/persistence/models.go` | DB model (`git_repos` table, `cloned_path`) |

## Architecture

### URI Detection and Git Repo Check

Two helpers are needed in `infrastructure/git/cloner.go`:

1. `isLocalURI(uri string) bool` — returns true when `uri` starts with `file://`.
2. `isGitRepo(path string) bool` — returns true when a `.git` directory (or file, for worktrees) exists inside `path`. Uses `os.Stat(filepath.Join(path, ".git"))`.

A local `file://` URI that is also a git repo is treated exactly like a remote git repo — all existing git operations apply. Only non-git local directories skip git ops.

### ClonePathFromURI

`ClonePathFromURI` already strips `file____` prefix from sanitized paths, but for local paths we want the working copy to point directly to the local directory. Change: if `isLocalURI(uri)`, strip the `file://` scheme and return the bare filesystem path directly.

```
file:///home/user/myproject  →  /home/user/myproject
```

This applies regardless of whether the directory is a git repo — the path is always used as-is.

### Clone

`Clone()` currently clones via the git adapter. For local URIs:
- Resolve the bare path by stripping `file://`
- Validate the directory exists (`os.Stat`)
- If `isGitRepo(path)`: proceed normally with `adapter.CloneRepository()` (git clone from local path)
- If not a git repo: skip clone, return the local path directly as the working copy

### Update

`Update()` currently does fetch + checkout + pull for branch tracking. For local URIs:
- If `isGitRepo(workingCopyPath)`: proceed normally with fetch/pull/checkout
- If not a git repo: skip all git operations, return stored working copy path as-is

### Ensure

`Ensure()` delegates to `adapter.EnsureRepository()`. For local URIs:
- If `isGitRepo(path)`: proceed normally
- If not a git repo: validate directory exists, return the local path

## Decisions

- **No new domain field needed.** Whether a repo is a plain local dir is determined at runtime by checking the URI scheme and `.git` presence. No DB flag needed.
- **URI sanitization is unchanged.** `sanitizeURIForPath()` already handles `file://` URIs for the clone-root case; the new `ClonePathFromURI` short-circuit bypasses sanitization for local paths.
- **Indexing pipeline unchanged.** The scanner reads from `workingCopy.path` — since that now points directly to the local dir, all downstream indexing (snippets, embeddings) works without modification.
- **Tracking config for plain local dirs.** Non-git directories have no branches/tags, so `trackingConfig` will be empty. The indexer must handle this gracefully (it already does for untracked repos).
- **Validation.** Check that the path exists and is a directory at Clone time and return a descriptive error if not.

## Codebase Patterns

- Uses functional options pattern for queries (`repository.WithRemoteURL(url)`).
- Git logic lives exclusively in `infrastructure/git/` — domain layer stays git-agnostic.
- The `Cloner` interface is injected into the application service, so no changes needed in the application layer.
- `ClonePathFromURI` is called from both the cloner and potentially from tests — keep it deterministic and pure.
