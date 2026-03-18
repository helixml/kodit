# Implementation Tasks

## Step 1: Write failing tests (TDD — red phase)

- [x] Add `TestIntegration_FileURI_GitRepo_WorkingCopyIsLocalPath` to `kodit_integration_test.go` — asserts `WorkingCopy().Path() == repoPath` after full pipeline
- [x] Add `TestIntegration_FileURI_GitRepo_SyncScansBranches` to `kodit_integration_test.go` — asserts commits are indexed when directory is a git repo
- [x] Add `TestIntegration_FileURI_NonGitDirectory_CompletesWithoutError` to `kodit_integration_test.go` — asserts pipeline completes without error for a plain directory
- [x] Tests fail for implementation reason (working copy path wrong / git clone attempted on local path)

## Step 2: Implement (green phase)

- [~] Add `isFileURI(uri string) bool` and `localPathFromFileURI(uri string) string` helpers in `infrastructure/git/cloner.go`
- [~] Add `isGitRepo(path string) bool` helper in `infrastructure/git/cloner.go` (uses `git rev-parse --git-dir`)
- [~] Update `RepositoryCloner.ClonePathFromURI()` to return the local path directly for `file://` URIs
- [~] Update `RepositoryCloner.Clone()` to skip `git clone` and return the local path for `file://` URIs
- [~] Guard the relocation fallback in `RepositoryCloner.Update()` so it never re-clones a `file://` URI
- [~] Update `RepositoryCloner.Update()` to skip fetch/pull only when the directory is **not** a git repo; let git repos proceed normally
- [ ] Run unit tests in `infrastructure/git/` and confirm they pass
