# Requirements: CSV File Parsing

## User Stories

**As a user**, I want CSV files in my repository to be indexed so that I can search for data contained in them.

**As a user**, I want only the meaningful string content of CSV files indexed (not numbers), so that search results are semantically useful rather than polluted with numeric noise.

## Acceptance Criteria

1. CSV files (`.csv` extension) are indexed when a repository is synced.
2. The text produced from a CSV file contains:
   - All column header names joined as a single string (if a header row is present).
   - All unique string values from every column whose values are non-numeric (at least one non-numeric, non-empty value in the column).
   - The raw text of the first five data rows.
3. Numeric columns are ignored entirely.
4. Duplicate string values within a column are deduplicated before indexing.
5. The produced text is passed to the standard chunking mechanism (no special chunker needed).
6. A unit test covers the CSV extractor: header parsing, string-only columns, deduplication, top-5 rows.
7. An end-to-end test (smoke or handler-level) verifies that a CSV file in a repo is chunked and persisted correctly.
8. A manual API test with a real CSV file confirms expected chunks are returned from search.
