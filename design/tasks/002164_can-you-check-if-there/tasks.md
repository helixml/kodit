# Implementation Tasks: Add Composite Index on Tasks Table for Dequeue Query

- [ ] Add composite index `idx_tasks_priority_created` (priority DESC, created_at ASC) via GORM tags on `TaskModel` in `infrastructure/persistence/models.go`
- [ ] Add composite index `idx_tasks_type_priority_created` (type, priority DESC, created_at ASC) via GORM tags on the same model (update the existing `type` field tag to participate in this index)
- [ ] Run `make check` to verify tests pass with the new tags
- [ ] Verify with `EXPLAIN` that `Dequeue` and `DequeueByOperation` queries use index scans (manual verification against a running database)
