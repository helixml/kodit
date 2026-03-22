# Implementation Tasks

- [x] Create `infrastructure/extraction/csv.go` with `ParseCSV(content []byte) (string, error)` using `encoding/csv`
- [~] Write unit tests in `infrastructure/extraction/csv_test.go` covering: header row, string-only columns, numeric columns skipped, deduplication, top-5 rows, no-header CSV, empty file
- [ ] Add `.csv: true` to `indexableExtensions` in `application/handler/indexing/chunk_files.go`
- [ ] Add CSV branch in `ChunkFiles.Execute` that reads content from git, calls `extraction.ParseCSV`, then chunks
- [ ] Add handler-level integration test in `chunk_files_test.go` for CSV files (assert chunks created, numeric columns absent)
- [ ] Run `make test PKG=./infrastructure/extraction/...` and `make test PKG=./application/handler/indexing/...` — all pass
- [ ] Test the API manually with a real CSV file: add a repo containing a CSV, wait for indexing, search and verify results match string columns only
- [ ] Run `make check` — no lint or vet errors
