# Design: Handler Test Coverage

## Approach

Follow the pattern already established in `enrichment/handler_test.go` and `commit/rescan_test.go`:

1. `testdb.New(t)` — real SQLite with full migrations.
2. Real persistence stores (from `infrastructure/persistence`).
3. `fakeEnricher` — returns `"enriched: " + request.ID()` for every request. Already defined in `enrichment/handler_test.go`; copy the minimal version per package.
4. `fakeTrackerFactory` / `fakeTracker` — no-op structs. Already exists in all handler packages; just copy.
5. For embedding/BM25: use `NewSQLiteEmbeddingStore` and `NewSQLiteBM25Store` directly — they work with the same SQLite DB.
6. For embedding domain service: use `recordingEmbedding` (captures docs) or a no-op; already in `indexing/ordering_test.go`.

## File Layout

Add tests alongside the untested handlers — one new test file per sub-package where tests are missing or incomplete:

```
application/handler/repository/handler_test.go   (clone, sync, delete)
application/handler/enrichment/handler_test.go   (already exists — extend it)
application/handler/indexing/handler_test.go     (bm25, embeddings, examples)
```

The enrichment package's existing `handler_test.go` already covers `CommitDescription` and `CreateSummary`. Add the missing handlers in the same file.

## Patterns Found in the Codebase

- **Idempotency**: Every handler checks for existing data first and skips if found. Each new test needs a "creates on first run" and "skips on second run" sub-test.
- **Fake LLM**: `fakeEnricher.Enrich()` returns one response per request. The handler_test.go in enrichment already defines this — copy it verbatim.
- **Fake git adapter**: `fakeGitAdapter` in enrichment's handler_test.go is a comprehensive fake. Clone/Sync/Delete tests need a fake `Cloner` and `Scanner` — define minimal interfaces per test.
- **BM25 store**: `persistence.NewSQLiteBM25Store(db, zerolog.Nop())` — pass the same `db` from `testdb.New(t)`.
- **Embedding store**: `persistence.NewSQLiteEmbeddingStore(db, persistence.TaskNameCode, zerolog.Nop())`.
- **Embedding domain service**: Use `recordingEmbedding` from `ordering_test.go` when you want to inspect what was sent; use a no-op that always succeeds otherwise.
- **Queue**: `service.Queue` is needed by Clone, Sync, Delete. Pass a queue built with the task store from the same `testdb.New(t)` DB: `persistence.NewTaskStore(db)` and `service.NewQueue(...)`.
- **Service.Enrichment**: The `Delete` handler uses `service.NewEnrichment(...)`. Pass `nil` for unused stores (BM25/embedding stores).
- **PrescribedOps**: `Sync` needs a `task.PrescribedOperations`. Use a real implementation from `task` package.

## Key Decisions

- **No mocks for stores** — use real SQLite; this validates the full persistence layer.
- **One assertion per behavior** — each test checks exactly one observable outcome (enrichment count, association count, record existence).
- **Prefer table-driven tests** when the same handler is exercised multiple times (e.g., idempotency).
- **Keep test helpers minimal** — inline setup rather than extracting helpers unless used 3+ times.

## Notes for Implementors

- The `fakeGitAdapter` in `enrichment/handler_test.go` implements the full `infraGit.Adapter` interface. For repository tests (clone/sync/delete) you only need `Cloner` and `Scanner` domain service interfaces — define small fakes inline.
- `ExtractExamples` reads files from disk (`os.ReadFile`). Use `t.TempDir()`, write a small `.go` or `.md` file, and point the fake repo working copy at that directory.
- `APIDocs` uses an `APIDocExtractor` interface — define a one-method fake returning a slice of enrichments.
- `ArchitectureDiscovery`, `Cookbook`, `DatabaseSchema` use single-method discoverer/gatherer interfaces — define tiny inline fakes.
- `Sync` enqueues tasks; verify they land in the task store after Execute, not that they are processed.
