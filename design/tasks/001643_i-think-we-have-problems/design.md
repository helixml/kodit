# Design: Fix PostgreSQL 65535 Parameter Limit in Bulk Saves

## Affected Methods

| Store | Method | Columns/row | Max rows before fail | Current pattern |
|---|---|---|---|---|
| `FileStore` | `SaveAll` | 8 | ~8,191 | `Create` (unbatched) |
| `CommitStore` | `SaveAll` | 8 | ~8,191 | `Save` (unbatched) |
| `BranchStore` | `SaveAll` | 6 | ~10,922 | `Create` (unbatched) |
| `TagStore` | `SaveAll` | 9 | ~7,281 | `Create` (unbatched) |

The embedding `SaveAll` methods (`SQLiteEmbeddingStore`, `VectorChordEmbeddingStore`) and `VectorChordBM25Store.batchInsert` already batch correctly at 100 records each.

However, there are two additional unbounded queries related to snippets/chunks that can also exceed the limit:

| Store | Method | Issue |
|---|---|---|
| `VectorChordBM25Store` | `existingIDs` | `SELECT ... WHERE snippet_id IN ?` with unbounded list |
| `VectorChordEmbeddingStore` | `Find` (via `filterNew`) | `SELECT ... WHERE snippet_id IN ?` with unbounded list |

These are called per-commit during indexing; a large commit with many snippets/chunks could pass 65535+ IDs.

## Fix

Use GORM's `CreateInBatches(models, batchSize)` for all four stores. A safe batch size is **1000** records per batch, giving a worst case of 9,000 parameters (9 cols × 1,000 rows) — well under 65,535.

**For `FileStore`, `BranchStore`, `TagStore`** (already use `Create` + `OnConflict`):
- Replace `Create(&models)` → `CreateInBatches(models, gitBatchSize)` keeping the `OnConflict` clause

**For `CommitStore`** (uses `Save` for upsert semantics):
- Replace `Save(&models)` with a loop that calls `Save` on slices of `gitBatchSize` records, or switch to `Create` + `OnConflict` + `CreateInBatches` for consistency

Add a shared constant `gitBatchSize = 1000` in a suitable file (e.g. `persistence/batch.go` or alongside the existing `saveAllBatchSize` in `embedding_store.go`).

**For `VectorChordBM25Store.existingIDs` and `VectorChordEmbeddingStore.Find` with `WithSnippetIDs`:**
- Both issue `WHERE snippet_id IN ?` with an unbounded slice
- Fix by chunking the ID list and executing multiple queries, then merging results
- A chunk size of 1000 IDs per query is safe (1 parameter per ID)

## Tests (TDD Red First)

Write integration tests in `infrastructure/persistence/` gated on a `POSTGRES_TEST_URL` environment variable (same pattern as `bm25_store_vectorchord_test.go`). Each test:

1. Opens a real PostgreSQL connection
2. Runs `AutoMigrate`
3. Inserts 10,000 records via the store's `SaveAll`
4. Asserts no error returned

These tests **currently fail** with `extended protocol limited to 65535 parameters` on `FileStore` and may fail on `CommitStore`/`BranchStore`/`TagStore` depending on repo size. After applying the fix they must pass.

One test file covers all stores: `infrastructure/persistence/bulk_save_postgres_test.go`.

For the BM25 and embedding store tests, 10,000 documents/embeddings should be inserted and the deduplication check (`existingIDs`) should also be called with 10,000 IDs to exercise the `IN ?` path.

## Codebase Patterns

- Existing batch constant: `saveAllBatchSize = 100` in `embedding_store.go` — keep separate since git stores can tolerate larger batches
- PostgreSQL test pattern: `POSTGRES_TEST_URL` env var, skip if absent, `AutoMigrate` before use — see `bm25_store_vectorchord_test.go`
- Existing `testdb.New(t)` helper creates in-memory SQLite — suitable only for non-PostgreSQL tests
