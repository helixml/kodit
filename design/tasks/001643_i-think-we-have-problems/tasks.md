# Implementation Tasks

- [ ] Write failing tests in `infrastructure/persistence/bulk_save_test.go` using `testdb.New(t)` (SQLite) with a GORM `BeforeCreate` callback that fails if any single statement exceeds 65535 bind parameters; cover `FileStore.SaveAll` (10k files), `CommitStore.SaveAll` (10k commits), `BranchStore.SaveAll` (10k branches), `TagStore.SaveAll` (10k tags), `VectorChordBM25Store` deduplication (10k IDs), and embedding store `SaveAll` (10k embeddings) — the git store tests should be red before the fix
- [ ] Add a `gitBatchSize = 1000` constant (e.g. in `infrastructure/persistence/batch.go` or near the existing `saveAllBatchSize`)
- [ ] Fix `FileStore.SaveAll`: replace `Create(&models)` with `CreateInBatches(models, gitBatchSize)` (keep `OnConflict` clause)
- [ ] Fix `BranchStore.SaveAll`: replace `Create(&models)` with `CreateInBatches(models, gitBatchSize)` (keep `OnConflict` clause)
- [ ] Fix `TagStore.SaveAll`: replace `Create(&models)` with `CreateInBatches(models, gitBatchSize)` (keep `OnConflict` clause)
- [ ] Fix `CommitStore.SaveAll`: replace unbatched `Save(&models)` with batched loop or `Create`+`OnConflict`+`CreateInBatches`
- [ ] Fix `VectorChordBM25Store.existingIDs`: chunk the ID list at 1000 IDs per `SELECT ... IN ?` query and merge results
- [ ] Fix `VectorChordEmbeddingStore.Find` with `WithSnippetIDs`: same chunked IN query approach
- [ ] Verify all tests now pass (`make test PKG=./infrastructure/persistence/...`)
- [ ] Run `make test` to confirm no existing tests broken
