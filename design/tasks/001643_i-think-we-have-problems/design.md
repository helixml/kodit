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

Tests run against SQLite only, using `testdb.New(t)`. SQLite has no 65535 parameter limit, so we cannot trigger the error directly. Instead, use a **GORM callback** to enforce the limit artificially.

### Parameter-limit callback

Register a GORM `BeforeCreate`/`BeforeUpdate` callback that counts bind parameters in the current statement's `SQL` and fails the test if any single statement exceeds a threshold (e.g., 65535). This is already a supported pattern in the codebase — `internal/database/repository_test.go` line 222 registers a `Callback().Query().After(...)` callback for SQL capture.

With the callback in place:
- **Before fix**: `FileStore.SaveAll(10000 files)` issues one INSERT with 80,000 params → callback fires → test fails ✓ red
- **After fix**: same call issues 10 batches of 1000 × 8 = 8,000 params each → callback stays silent → test passes ✓ green

Each test:
1. Opens SQLite via `testdb.New(t)`
2. Registers the parameter-limit callback on the GORM session
3. Calls the store's `SaveAll` (or `Index`/`existingIDs`) with 10,000 records/IDs
4. Asserts no error

One test file: `infrastructure/persistence/bulk_save_test.go`.

## Codebase Patterns

- Existing batch constant: `saveAllBatchSize = 100` in `embedding_store.go` — keep separate since git stores can tolerate larger batches
- Existing `testdb.New(t)` helper creates in-memory SQLite with all migrations applied — use this for all tests
- GORM callback pattern already used in tests: `db.Callback().Query().After("gorm:query").Register("test:capture", func(db *gorm.DB) {...})` — see `internal/database/repository_test.go:222`
