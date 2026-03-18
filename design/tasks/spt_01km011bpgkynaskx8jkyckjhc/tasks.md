# Implementation Tasks

## Step 1: Write failing tests (TDD — red phase)

- [~] Add `TestIntegration_FileURI_GitRepo_WorkingCopyIsLocalPath` to `kodit_integration_test.go` — asserts `WorkingCopy().Path() == repoPath` after full pipeline
- [~] Add `TestIntegration_FileURI_GitRepo_SyncScansBranches` to `kodit_integration_test.go` — asserts commits are indexed when directory is a git repo
- [~] Add `TestIntegration_FileURI_PlainDirectory_CompletesWithoutError` to `kodit_integration_test.go` — asserts pipeline completes without error for a plain directory
- [~] Run `make test` and confirm all three new tests fail

## Step 2: Implement (green phase)

- [ ] Add `isFileURI(uri string) bool` and `localPathFromFileURI(uri string) string` helpers in `infrastructure/git/cloner.go`
- [ ] Add `isGitRepo(path string) bool` helper in `infrastructure/git/cloner.go` (checks for `.git` entry via `os.Stat`)
- [ ] Update `RepositoryCloner.ClonePathFromURI()` to return the local path directly for `file://` URIs
- [ ] Update `RepositoryCloner.Clone()` to skip `git clone` and return the local path for `file://` URIs
- [ ] Guard the relocation fallback in `RepositoryCloner.Update()` so it never re-clones a `file://` URI
- [ ] Update `RepositoryCloner.Update()` to skip fetch/pull only when the directory is **not** a git repo; let git repos proceed normally
- [ ] Run `make test` and confirm all three new tests pass and no existing tests regress
