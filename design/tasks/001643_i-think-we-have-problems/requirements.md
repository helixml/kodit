# Requirements: Fix PostgreSQL 65535 Parameter Limit in Bulk Saves

## Problem

PostgreSQL limits a single query to 65535 bind parameters. When indexing large repos (e.g. Apache Airflow), `FileStore.SaveAll` and similar methods pass all records in a single `INSERT`, blowing past the limit:

```
error="extended protocol limited to 65535 parameters"
```

The same unbatched pattern exists in `CommitStore.SaveAll`, `BranchStore.SaveAll`, and `TagStore.SaveAll`.

## User Stories

- As a user indexing a large repository, I want `kodit` to complete indexing without errors regardless of how many files/commits/branches/tags are in each batch.

## Acceptance Criteria

- [ ] Inserting 10,000 git commit files succeeds against PostgreSQL
- [ ] Inserting 10,000 commits succeeds against PostgreSQL
- [ ] Inserting 10,000 branches succeeds against PostgreSQL
- [ ] Inserting 10,000 tags succeeds against PostgreSQL
- [ ] Each bulk save method processes records in chunks ≤ a safe batch size
- [ ] All existing tests continue to pass
