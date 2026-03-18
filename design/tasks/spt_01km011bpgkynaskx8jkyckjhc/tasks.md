# Implementation Tasks

- [ ] Add `isFileURI(uri string) bool` and `localPathFromFileURI(uri string) string` helpers in `infrastructure/git/cloner.go`
- [ ] Update `RepositoryCloner.ClonePathFromURI()` to return the local path directly for `file://` URIs
- [ ] Update `RepositoryCloner.Clone()` to skip `git clone` and return the local path for `file://` URIs
- [ ] Add `isGitRepo(path string) bool` helper in `infrastructure/git/cloner.go` (checks for `.git` entry)
- [ ] Update `RepositoryCloner.Update()` to skip fetch/pull only when the `file://` directory is **not** a git repo; let git repos proceed normally
- [ ] Guard the relocation fallback in `RepositoryCloner.Update()` so it never re-clones a `file://` URI
- [ ] Add unit tests for the new `file://` paths in `cloner_test.go` (Clone, Update, ClonePathFromURI)
- [ ] Verify end-to-end: add a `file:///` repo via API, confirm `cloned_path` is set correctly and sync completes without git errors
