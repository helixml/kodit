# Design: CSV File Parsing

## Architecture

CSV is a text format read directly from git (not a binary document needing tabula). The implementation adds a lightweight CSV parser in `infrastructure/extraction/csv.go` and adds a third branch in the `ChunkFiles` handler for `.csv` files.

### New file: `infrastructure/extraction/csv.go`

```go
// ParseCSV converts CSV content into an indexable string:
// 1. Header row (all column names joined by space)
// 2. Unique string values from non-numeric columns
// 3. Top-5 data rows verbatim
func ParseCSV(content []byte) (string, error)
```

**Column classification**: a column is "string" if it has at least one value that cannot be parsed as a float64 (using `strconv.ParseFloat`). Empty values are skipped in classification and in deduplication.

**Output format** (all sections separated by newlines):
```
Headers: col1 col2 col3
Values: foo bar baz qux
Top rows:
foo,42,bar
...
```

Use Go's `encoding/csv` standard library — no new dependencies.

### Changes to `application/handler/indexing/chunk_files.go`

Add `.csv: true` to `indexableExtensions`.

Add a new branch inside the file loop (after the document check, before the plain-text branch):

```go
if ext == ".csv" {
    content, readErr := h.fileContent.FileContent(ctx, clonedPath, cp.CommitSHA(), relPath)
    // handle error...
    text, parseErr := extraction.ParseCSV(content)
    // handle error, skip empty...
    textChunks, chunkErr = chunking.NewTextChunks(text, h.params)
    // handle error...
}
```

This keeps the existing document and plain-text paths unchanged.

### No changes to handlers.go or any store

No new interface, no new dependency injection. `ParseCSV` is a pure function.

## Key Decisions

- **Pure function over interface**: CSV parsing doesn't need a `DocumentTextSource` instance; a package-level function is simpler and avoids changing `NewChunkFiles` signatures.
- **Standard library only**: `encoding/csv` handles quoting, escaping, and line endings correctly. No third-party CSV library needed.
- **Float64 as numeric test**: `strconv.ParseFloat(v, 64) == nil` treats integers, floats, and scientific notation as numeric. This is the simplest correct heuristic.
- **Deduplication via map**: `map[string]struct{}` gives O(1) dedup without sorting.
- **Top-5 rows verbatim**: preserves the raw CSV row text (re-encoded via `csv.Writer`) so the original formatting is searchable.

## Codebase Patterns Found

- Extraction helpers live in `infrastructure/extraction/` — follow the same package.
- `chunk_files.go` uses `strings.ToLower(filepath.Ext(f.Path()))` for extension checks — match this pattern.
- Tests use `testdb.New(t)` for in-memory DB, `fakeGitAdapter` for file content, and assert on `enrichmentStore.Find(...)` results.
- `make test PKG=./infrastructure/extraction/...` and `make test PKG=./application/handler/indexing/...` run targeted tests.
- Smoke tests in `test/smoke/smoke_test.go` test against a live server — the CSV E2E test should be a handler-level integration test (same pattern as `TestChunkFiles_CreatesEnrichmentsForTextFiles`).
