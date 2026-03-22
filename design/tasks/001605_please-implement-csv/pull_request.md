# Pull Request: CSV File Indexing Support

## Summary

- Add `infrastructure/extraction/csv.go` with `ParseCSV(content []byte) (string, error)` that converts CSV content into an indexable string: header names, deduplicated string-only column values (numeric columns excluded), and the first 5 raw rows for context
- Add `.csv` to `indexableExtensions` in `chunk_files.go` and a new CSV branch in `ChunkFiles.Execute` that reads content from git, parses it, then chunks the result using the existing text chunking pipeline
- Add unit tests covering header row, string-only columns, numeric column exclusion, deduplication, top-5 rows, no-header CSV, and empty file; add handler-level integration test asserting chunks are created and numeric values are absent

## Test plan

- [x] `make test PKG=./infrastructure/extraction/...` — all CSV unit tests pass
- [x] `make test PKG=./application/handler/indexing/...` — all handler tests pass including `TestChunkFiles_ParsesCSVFiles`
- [x] `make check PKG=./application/handler/indexing/...` — no lint or vet errors
- [x] `make check PKG=./infrastructure/extraction/...` — no lint or vet errors
- [x] Manual end-to-end: added a git repo containing `employees.csv` to a running kodit instance, triggered sync, verified enrichment chunk content contains string column values (`Alice Smith`, `Engineering`, `San Francisco`) but NOT numeric values (`32`, `95000`)

🤖 Generated with [Claude Code](https://claude.com/claude-code)
