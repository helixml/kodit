# Implementation Tasks

- [ ] Add `isLocalURI(uri string) bool` helper in `infrastructure/git/cloner.go` — true when uri has `file://` scheme
- [ ] Add `isGitRepo(path string) bool` helper in `infrastructure/git/cloner.go` — true when `.git` exists inside path
- [ ] Update `ClonePathFromURI` in `infrastructure/git/cloner.go`: if `isLocalURI`, strip `file://` and return the bare local path directly
- [ ] Update `Clone` in `infrastructure/git/cloner.go`: if `isLocalURI` and NOT `isGitRepo`, validate dir exists and return local path without cloning; if it is a git repo, proceed normally
- [ ] Update `Update` in `infrastructure/git/cloner.go`: if `isLocalURI` and NOT `isGitRepo`, skip all git fetch/pull/checkout and return stored working copy path
- [ ] Update `Ensure` in `infrastructure/git/cloner.go`: if `isLocalURI` and NOT `isGitRepo`, validate dir exists and return local path
- [ ] Add unit tests for `isLocalURI`, `isGitRepo`, and the updated `ClonePathFromURI` logic
- [ ] Add integration/e2e test: create repo with `file:///tmp/testdir` (non-git), assert no git ops called and `cloned_path` equals `/tmp/testdir`
- [ ] Add integration/e2e test: create repo with `file:///tmp/gitrepo` (valid git repo), assert git ops proceed normally
- [ ] Verify indexing pipeline works end-to-end with a plain local directory (manual or automated test)
