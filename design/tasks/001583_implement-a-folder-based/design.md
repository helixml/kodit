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

### URI Detection

A repository is "local" if its `remoteURI` starts with `file://`. This check belongs in `infrastructure/git/cloner.go` since that's where git decisions are made. A small helper (`isLocalURI(uri string) bool`) will be added there.

### ClonePathFromURI

`ClonePathFromURI` already strips `file____` prefix from sanitized paths, but for local repos we want the working copy path to be the real local path, not a derived subdirectory. Change: if `isLocalURI(uri)`, strip the `file://` scheme and return the bare filesystem path directly.

```
file:///home/user/myproject  →  /home/user/myproject
```

### Clone

`Clone()` currently clones via the git adapter. For local URIs:
- Skip `adapter.CloneRepository()`
- Validate the directory exists (`os.Stat`)
- Return the local path as the working copy path

### Update

`Update()` currently does fetch + checkout + pull for branch tracking. For local URIs:
- Skip all git operations
- Return the stored working copy path as-is (directory is always "up to date")

### Ensure

`Ensure()` delegates to `adapter.EnsureRepository()`. For local URIs:
- Skip the adapter call
- Validate directory exists and return the local path

## Decisions

- **No new domain field needed.** Whether a repo is local is fully determined by the URI scheme. No boolean flag in the domain model or DB is needed.
- **URI sanitization is unchanged.** `sanitizeURIForPath()` already handles `file://` URIs for the clone-root case; the new `ClonePathFromURI` short-circuit bypasses sanitization for local repos.
- **Indexing pipeline unchanged.** The scanner reads from `workingCopy.path` — since that now points directly to the local dir, all downstream indexing (snippets, embeddings) works without modification.
- **Tracking config for local repos.** Local directories have no branches/tags, so `trackingConfig` will be empty (no branch/tag/commit). The indexer must handle this gracefully (it already does for untracked repos).
- **Validation.** Check that the path exists and is a directory at Clone time and return a descriptive error if not.

## Codebase Patterns

- Uses functional options pattern for queries (`repository.WithRemoteURL(url)`).
- Git logic lives exclusively in `infrastructure/git/` — domain layer stays git-agnostic.
- The `Cloner` interface is injected into the application service, so no changes needed in the application layer.
- `ClonePathFromURI` is called from both the cloner and potentially from tests — keep it deterministic and pure.
