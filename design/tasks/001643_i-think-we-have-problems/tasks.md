# Implementation Tasks

- [ ] Write failing integration tests in `infrastructure/persistence/bulk_save_postgres_test.go` (gated on `POSTGRES_TEST_URL`) that insert 10,000 records each via `FileStore.SaveAll`, `CommitStore.SaveAll`, `BranchStore.SaveAll`, `TagStore.SaveAll` and assert no error — these should fail with the current unbatched code
- [ ] Add a `gitBatchSize = 1000` constant (e.g. in `infrastructure/persistence/batch.go` or near the existing `saveAllBatchSize`)
- [ ] Fix `FileStore.SaveAll`: replace `Create(&models)` with `CreateInBatches(models, gitBatchSize)` (keep `OnConflict` clause)
- [ ] Fix `BranchStore.SaveAll`: replace `Create(&models)` with `CreateInBatches(models, gitBatchSize)` (keep `OnConflict` clause)
- [ ] Fix `TagStore.SaveAll`: replace `Create(&models)` with `CreateInBatches(models, gitBatchSize)` (keep `OnConflict` clause)
- [ ] Fix `CommitStore.SaveAll`: replace unbatched `Save(&models)` with batched loop or `Create`+`OnConflict`+`CreateInBatches`
- [ ] Verify all four integration tests now pass against PostgreSQL
- [ ] Run `make test` to confirm no existing tests broken
