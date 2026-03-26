# Implementation Tasks

- [ ] Write failing integration tests in `infrastructure/persistence/bulk_save_postgres_test.go` (gated on `POSTGRES_TEST_URL`) covering: `FileStore.SaveAll` (10k files), `CommitStore.SaveAll` (10k commits), `BranchStore.SaveAll` (10k branches), `TagStore.SaveAll` (10k tags), `VectorChordBM25Store.Index` (10k documents including the deduplication check), and `VectorChordEmbeddingStore.SaveAll` (10k embeddings) — the git store tests should fail with the current unbatched code
- [ ] Add a `gitBatchSize = 1000` constant (e.g. in `infrastructure/persistence/batch.go` or near the existing `saveAllBatchSize`)
- [ ] Fix `FileStore.SaveAll`: replace `Create(&models)` with `CreateInBatches(models, gitBatchSize)` (keep `OnConflict` clause)
- [ ] Fix `BranchStore.SaveAll`: replace `Create(&models)` with `CreateInBatches(models, gitBatchSize)` (keep `OnConflict` clause)
- [ ] Fix `TagStore.SaveAll`: replace `Create(&models)` with `CreateInBatches(models, gitBatchSize)` (keep `OnConflict` clause)
- [ ] Fix `CommitStore.SaveAll`: replace unbatched `Save(&models)` with batched loop or `Create`+`OnConflict`+`CreateInBatches`
- [ ] Fix `VectorChordBM25Store.existingIDs`: chunk the ID list at 1000 IDs per `SELECT ... IN ?` query and merge results
- [ ] Fix `VectorChordEmbeddingStore.Find` with `WithSnippetIDs`: same chunked IN query approach
- [ ] Verify all integration tests now pass against PostgreSQL
- [ ] Run `make test` to confirm no existing tests broken
