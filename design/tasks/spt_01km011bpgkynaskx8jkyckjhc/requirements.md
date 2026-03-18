# Requirements: Folder-Based Repository Creation Mode

## User Stories

**As a user, I want to pass a `file:///path/to/dir` URI when adding a repository so that Kodit indexes a local directory without cloning it.**

### Acceptance Criteria

1. When a repository is added with a URI beginning with `file://`, Kodit:
   - Accepts the URI without error
   - Does **not** perform a git clone (the directory already exists locally)
   - Sets the `cloned_path` in the database to the local filesystem path extracted from the URI (e.g. `file:///home/user/project` → `/home/user/project`)

2. When syncing a `file://` repository, Kodit:
   - If git recognises the directory as a git repository (i.e. `git rev-parse --git-dir` succeeds), runs `git fetch`, `git pull`, and scans branches/commits as usual
   - If git does not recognise the directory as a git repository, skips all git operations and proceeds to index whatever files are present

3. A `file://` repository behaves identically to a cloned git repository in all downstream steps (commit scanning, enrichment, MCP search), to the extent the directory supports it.

4. Non-`file://` URIs continue to work exactly as before — no regression.

---

## TDD: Tests to Write First

Use the existing patterns from `kodit_integration_test.go` (full pipeline with a real worker) and `application/service/repository_test.go` (service-level with real stores). Tests must be written and confirmed to **fail** before implementation begins.

### Test 1 — `file://` git repo: `cloned_path` is the original path, not a clone dir subdirectory

```
// TestIntegration_FileURI_GitRepo_WorkingCopyIsLocalPath
// In: kodit_integration_test.go
//
// 1. createTestGitRepo(t) → repoPath
// 2. client.Repositories.Add(ctx, fileURI(repoPath))
// 3. waitForTasks(...)
// 4. Get the repo from the store
// 5. Assert repo.HasWorkingCopy() == true
// 6. Assert repo.WorkingCopy().Path() == repoPath  (not a subdirectory of cloneDir)
```

### Test 2 — `file://` git repo: sync still scans branches and commits

```
// TestIntegration_FileURI_GitRepo_SyncScansBranches
// In: kodit_integration_test.go
//
// 1. createTestGitRepo(t) → repoPath
// 2. client.Repositories.Add(ctx, fileURI(repoPath), branch)
// 3. waitForTasks(...)
// 4. client.Commits.Find(ctx, WithRepoID(repo.ID()))
// 5. Assert commits are non-empty  (git scanning worked on the local repo)
```

### Test 3 — `file://` directory not recognised by git: indexing completes without error

```
// TestIntegration_FileURI_NonGitDirectory_CompletesWithoutError
// In: kodit_integration_test.go
//
// 1. t.TempDir() → plainDir; write a couple of source files into it (no git init)
//    — git rev-parse --git-dir will fail for this directory
// 2. client.Repositories.Add(ctx, fileURI(plainDir))
// 3. waitForTasks(...)  — must not timeout or leave failed tasks
// 4. Get the repo from the store
// 5. Assert repo.HasWorkingCopy() == true
// 6. Assert repo.WorkingCopy().Path() == plainDir
```
