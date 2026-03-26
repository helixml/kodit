# Requirements: Fix PostgreSQL 65535 Parameter Limit in Bulk Saves

## Problem

PostgreSQL limits a single query to 65535 bind parameters. When indexing large repos (e.g. Apache Airflow), `FileStore.SaveAll` and similar methods pass all records in a single `INSERT`, blowing past the limit:

```
error="extended protocol limited to 65535 parameters"
```

The same unbatched pattern exists in `CommitStore.SaveAll`, `BranchStore.SaveAll`, and `TagStore.SaveAll`.

Additionally, `VectorChordBM25Store.existingIDs` and the embedding store's `Find` with `WithSnippetIDs` pass unbounded lists of snippet IDs in `IN ?` clauses, which can also exceed the limit when a commit produces many snippets/chunks.

## User Stories

- As a user indexing a large repository, I want `kodit` to complete indexing without errors regardless of how many files/commits/branches/tags/snippets are in each batch.

## Acceptance Criteria

- [ ] `FileStore.SaveAll` with 10,000 files never issues a single SQL statement with more than 65,535 bind parameters
- [ ] `CommitStore.SaveAll` with 10,000 commits never exceeds the parameter limit per statement
- [ ] `BranchStore.SaveAll` with 10,000 branches never exceeds the parameter limit per statement
- [ ] `TagStore.SaveAll` with 10,000 tags never exceeds the parameter limit per statement
- [ ] BM25 deduplication check with 10,000 snippet IDs never issues a single `IN ?` with more than 65,535 parameters
- [ ] Embedding `SaveAll` with 10,000 vectors never exceeds the parameter limit per statement
- [ ] Each bulk save method processes records in chunks ≤ a safe batch size
- [ ] All existing tests continue to pass
