# Requirements: Add Composite Index on Tasks Table for Dequeue Query

## Background

The `Dequeue` and `DequeueByOperation` methods in `kodit` query the `tasks` table with:

```sql
SELECT * FROM "tasks" ORDER BY priority DESC, created_at ASC LIMIT 1
```

The `TaskModel` (`infrastructure/persistence/models.go:171`) has no index on `priority` or `created_at`. Only `type` and `dedup_key` are indexed. This forces PostgreSQL to do a full table sort on every dequeue call.

## User Stories

- As a system operator, I want the task dequeue query to use an index so that latency stays low as the task queue grows.

## Acceptance Criteria

- [ ] A composite index exists on the `tasks` table covering `(priority DESC, created_at ASC)`
- [ ] The `Dequeue` query (`ORDER BY priority DESC, created_at ASC LIMIT 1`) uses an index scan instead of a sequential scan + sort
- [ ] The `DequeueByOperation` query (`WHERE type = ? ORDER BY priority DESC, created_at ASC LIMIT 1`) also benefits — consider a composite index on `(type, priority DESC, created_at ASC)`
- [ ] Existing tests pass (`make check`)
- [ ] Index is added via GORM tags (consistent with the project's "GORM AutoMigrate only — no SQL migration files" rule)
