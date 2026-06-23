perf: add composite indexes on tasks table

## Summary
The `Dequeue` and `DequeueByOperation` queries order by `priority DESC, created_at ASC` but the `tasks` table had no matching index, causing a full table scan + sort on every dequeue call.

## Changes
- Added composite index `idx_tasks_priority_created` (priority DESC, created_at ASC) for `Dequeue`
- Added composite index `idx_tasks_type_priority_created` (type, priority DESC, created_at ASC) for `DequeueByOperation`
- Both indexes are defined via GORM struct tags on `TaskModel` and will be created automatically by AutoMigrate

## Testing
- `golangci-lint` passes clean (0 issues)
- Indexes will be created on next application startup via GORM AutoMigrate
- Verify with `EXPLAIN ANALYZE` after deployment that both queries use index scans
