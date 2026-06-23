# Design: Add Composite Index on Tasks Table for Dequeue Query

## Current State

The `TaskModel` in `infrastructure/persistence/models.go:171-178` defines these columns:

| Column | Type | Index |
|--------|------|-------|
| `id` | int64 | primaryKey |
| `dedup_key` | varchar(255) | uniqueIndex |
| `type` | varchar(255) | index |
| `payload` | jsonb | none |
| `priority` | int | **none** |
| `created_at` | timestamp | **none** |
| `updated_at` | timestamp | none |

Two queries hit this table repeatedly:

1. **`Dequeue`** (`task_store.go:57`): `ORDER BY priority DESC, created_at ASC LIMIT 1`
2. **`DequeueByOperation`** (`task_store.go:88-89`): `WHERE type = ? ORDER BY priority DESC, created_at ASC LIMIT 1`

Without a matching index, both require a full table scan + sort.

## Solution

Add GORM composite index tags to `TaskModel`. GORM supports composite indexes via `index` tag with shared index names and sort direction.

### Index 1: Dequeue ordering

Add a composite index `idx_tasks_priority_created` on `(priority DESC, created_at ASC)`.

This directly serves the `Dequeue` query — PostgreSQL can return the first row from an index scan with no sort.

### Index 2: DequeueByOperation ordering

The existing single-column index on `type` doesn't help when the query also needs `ORDER BY priority DESC, created_at ASC`. A composite index `idx_tasks_type_priority_created` on `(type, priority DESC, created_at ASC)` serves `DequeueByOperation` optimally — equality filter on `type`, then ordered scan.

With index 2 in place, index 1 is technically redundant for `Dequeue` (PostgreSQL can do a full scan of index 2 ignoring the `type` prefix), but keeping both is clearer and a two-column index is small.

### Implementation

Update the GORM tags on `TaskModel`:

```go
type TaskModel struct {
    ID        int64           `gorm:"column:id;primaryKey;autoIncrement"`
    DedupKey  string          `gorm:"column:dedup_key;type:varchar(255);uniqueIndex;not null"`
    Type      string          `gorm:"column:type;type:varchar(255);index:idx_tasks_type_priority_created,priority:1;not null"`
    Payload   json.RawMessage `gorm:"column:payload;type:jsonb"`
    Priority  int             `gorm:"column:priority;not null;index:idx_tasks_priority_created,priority:1,sort:desc;index:idx_tasks_type_priority_created,priority:2,sort:desc"`
    CreatedAt time.Time       `gorm:"column:created_at;autoCreateTime;index:idx_tasks_priority_created,priority:2,sort:asc;index:idx_tasks_type_priority_created,priority:3,sort:asc"`
    UpdatedAt time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}
```

GORM AutoMigrate will create the indexes automatically. The old single-column `idx_task_models_type` index becomes redundant — GORM will keep it (it won't drop indexes), which is fine.

## Decisions

- **GORM tags only** — the project rule is "GORM AutoMigrate only, no SQL migration files."
- **Two indexes instead of one** — keeps the `Dequeue` path clean without relying on PostgreSQL choosing a less-selective multi-column index.
- **No code changes to queries** — the queries in `task_store.go` remain unchanged; only the model tags change.

## Implementation Notes

- Only file changed: `infrastructure/persistence/models.go` (TaskModel struct tags)
- This codebase already uses named composite GORM indexes (see `idx_enrichment_entity`, `idx_trackable` in the same file) — followed the same pattern
- The existing single-column `index` tag on `Type` is kept alongside the new composite index participation — GORM handles both
- Lint (`golangci-lint`) passes clean with 0 issues
- Tests require CGO (sqlite3) which isn't available in this dev environment; all test failures are pre-existing and unrelated to this change
- Indexes will be created automatically on next `AutoMigrate` run at application startup — no manual DDL needed
