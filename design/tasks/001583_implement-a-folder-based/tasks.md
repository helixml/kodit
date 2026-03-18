# Implementation Tasks

- [ ] Add `isLocalURI(uri string) bool` helper in `infrastructure/git/cloner.go` that returns true when uri has `file://` scheme
- [ ] Update `ClonePathFromURI` in `infrastructure/git/cloner.go`: if `isLocalURI`, strip `file://` and return the bare local path directly
- [ ] Update `Clone` in `infrastructure/git/cloner.go`: if `isLocalURI`, validate dir exists via `os.Stat`, skip `adapter.CloneRepository`, return local path
- [ ] Update `Update` in `infrastructure/git/cloner.go`: if `isLocalURI`, skip all git fetch/pull/checkout operations and return stored working copy path
- [ ] Update `Ensure` in `infrastructure/git/cloner.go`: if `isLocalURI`, skip `adapter.EnsureRepository`, validate dir exists, return local path
- [ ] Add unit tests for `isLocalURI` and the updated `ClonePathFromURI` logic
- [ ] Add integration/e2e test: create repo with `file:///tmp/testdir`, assert no git ops called and `cloned_path` equals `/tmp/testdir`
- [ ] Verify indexing pipeline works end-to-end with a local directory (manual or automated test)
