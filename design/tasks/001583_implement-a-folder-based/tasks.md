# Implementation Tasks

- [ ] Write a failing application service e2e test in `application/service/` that calls `Repository.Add()` with a `file://` URI pointing to a temp plain directory, and asserts: no git clone is called, the saved working copy path equals the local directory path
- [ ] Add `isLocalURI(uri string) bool` helper in `infrastructure/git/cloner.go` — true when uri has `file://` scheme
- [ ] Add `isGitRepo(path string) bool` helper in `infrastructure/git/cloner.go` — true when `.git` exists inside path
- [ ] Update `ClonePathFromURI` in `infrastructure/git/cloner.go`: if `isLocalURI`, strip `file://` and return the bare local path directly
- [ ] Update `Clone` in `infrastructure/git/cloner.go`: if `isLocalURI` and NOT `isGitRepo`, validate dir exists and return local path without cloning; if it is a git repo, proceed normally
- [ ] Update `Update` in `infrastructure/git/cloner.go`: if `isLocalURI` and NOT `isGitRepo`, skip all git fetch/pull/checkout and return stored working copy path
- [ ] Update `Ensure` in `infrastructure/git/cloner.go`: if `isLocalURI` and NOT `isGitRepo`, validate dir exists and return local path
- [ ] Confirm the e2e test passes; add unit tests for `isLocalURI`, `isGitRepo`, and `ClonePathFromURI`
