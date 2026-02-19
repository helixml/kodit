# Repository Design Review

## Executive Summary

Three stores violate the generic repository pattern: **CommitIndexStore**, **TaskStore**, and **StatusStore**. One is dead code. The other two maintain separate `db`/`mapper` fields, define custom query methods, and expose far too many public methods. Additionally, several interface methods across these stores have zero callers and should be pruned.

---

## Finding 1: CommitIndexStore Is Dead Code

**File:** `infrastructure/persistence/snippet_store.go`
**Interface:** `domain/snippet/store.go`

The `CommitIndexStore` interface defines 4 methods (`Get`, `Save`, `Delete`, `Exists`), the implementation uses raw GORM queries with separate `db`/`mapper` fields, and **none of these methods are called anywhere in the codebase**.

### Action

Delete `CommitIndexStore` entirely:
- Remove the interface from `domain/snippet/store.go`
- Remove the implementation from `infrastructure/persistence/snippet_store.go`
- Remove any mapper or model code that is only used by this store

---

## Finding 2: TaskStore Doesn't Embed Repository

**File:** `infrastructure/persistence/task_store.go` (lines 16-254)
**Interface:** `domain/task/store.go` (lines 10-47)

### Violations

| Issue | Detail |
|-------|--------|
| No embedding | Stores separate `db` and `mapper` fields instead of embedding `database.Repository` |
| 11 public methods | Violates the "fewer than five" guideline |
| Custom `Get(id)` | Should be `FindOne(ctx, repository.WithID(id))` |
| Custom `FindAll()` | Should be `Find(ctx)` with ordering options |
| Custom `FindPending()` | Should be `Find(ctx, options...)` — ordering pushed into options |
| Custom `CountPending()` | Should be `Count(ctx, options...)` |
| Custom `Exists(id)` | Should be `Exists(ctx, repository.WithID(id))` |
| Raw WHERE clauses | Uses `.Where("id = ?", ...)` instead of `WithCondition` |

### Dead Methods (Zero Callers)

- `Exists(ctx, id)` — never called
- `DeleteAll(ctx)` — never called
- `SaveBulk(ctx, tasks)` — never called
- `DequeueByOperation(ctx, op)` — never called

### Mapper Constraint

`TaskMapper.ToDomain` and `TaskMapper.ToModel` both return errors because they marshal/unmarshal a JSON payload. The generic `EntityMapper[D, E]` interface requires infallible signatures: `ToDomain(E) D` and `ToModel(D) E`.

**Resolution:** Restructure `task.Task` to store the serialized payload bytes internally alongside the parsed map:
- `NewTask(op, priority, payload map[string]any)` marshals eagerly and returns `(Task, error)` — fail fast at construction
- `Payload() map[string]any` unmarshals lazily from stored bytes
- The mapper becomes infallible: it copies bytes, not maps
- `NewTaskFromRecord(id, dedupKey, op, priority, payloadBytes, createdAt, updatedAt) Task` provides an infallible reconstruction path for the mapper

### Action

1. **Remove dead methods** from the `TaskStore` interface and implementation: `Exists`, `DeleteAll`, `SaveBulk`, `DequeueByOperation`
2. **Restructure `task.Task`** to store raw payload bytes, making `TaskMapper` infallible
3. **Make `TaskMapper` implement `database.EntityMapper[task.Task, TaskModel]`**
4. **Embed `database.Repository[task.Task, TaskModel]`** in `TaskStore`
5. **Define typed options** in `domain/task/options.go`:
   - `WithOperation(op)` → `WithCondition("type", op.String())`
   - `WithDedupKey(key)` → `WithCondition("dedup_key", key)`
   - Default ordering: `WithPriorityOrder()` → `WithOrderDesc("priority")` + `WithOrderAsc("created_at")`
6. **Update callers** to use generic methods:
   - `Queue.Get(id)` → `store.FindOne(ctx, repository.WithID(id))`
   - `Queue.List()` → `store.Find(ctx, task.WithPriorityOrder(), ...options)` with `WithOperation` for filtering instead of in-memory filtering
   - `Queue.Count()` → `store.Count(ctx)`
   - `Queue.DrainForRepository()` → `store.Find(ctx, task.WithPriorityOrder())` (payload filtering must stay in-memory since repo_id is inside JSON)
7. **Keep as custom methods** on the store (justified):
   - `Save(ctx, task) (Task, error)` — uses `ON CONFLICT (dedup_key)` upsert logic
   - `Delete(ctx, task) error` — standard delete by ID
   - `Dequeue(ctx) (Task, bool, error)` — transactional read-then-delete that cannot be expressed via options

### Resulting Interface

```go
type TaskStore interface {
    // From embedded Repository:
    // Find(ctx, ...options) ([]Task, error)
    // FindOne(ctx, ...options) (Task, error)
    // Exists(ctx, ...options) (bool, error)
    // Count(ctx, ...options) (int64, error)
    // DeleteBy(ctx, ...options) error

    Save(ctx context.Context, task Task) (Task, error)
    Delete(ctx context.Context, task Task) error
    Dequeue(ctx context.Context) (Task, bool, error)
}
```

---

## Finding 3: StatusStore Doesn't Embed Repository

**File:** `infrastructure/persistence/task_store.go` (lines 257-422)
**Interface:** `domain/task/store.go` (lines 49-80)

### Violations

| Issue | Detail |
|-------|--------|
| No embedding | Stores separate `db`/`mapper` fields |
| 8 public methods | Violates the "fewer than five" guideline |
| Custom `Get(id)` | Should be `FindOne(ctx, repository.WithID(id))` |
| Custom `FindByTrackable()` | Should be `Find(ctx, WithTrackableType(...), WithTrackableID(...))` |
| Custom `Count()` | Should be `Count(ctx)` from embedding |
| Custom `DeleteByTrackable()` | Should be `DeleteBy(ctx, WithTrackableType(...), WithTrackableID(...))` |
| Raw WHERE clauses | Uses `.Where("trackable_type = ? AND ...")` instead of options |

### Dead Methods (Zero Callers)

- `Get(ctx, id)` — never called
- `SaveBulk(ctx, statuses)` — never called
- `Delete(ctx, status)` — never called

### Mapper Status

`TaskStatusMapper` is already infallible — `ToDomain(E) D` and `ToModel(D) E` with no error returns. It already conforms to `database.EntityMapper[task.Status, TaskStatusModel]`. No restructuring needed.

### Action

1. **Remove dead methods** from the `StatusStore` interface and implementation: `Get`, `SaveBulk`, `Delete`
2. **Embed `database.Repository[task.Status, TaskStatusModel]`** in `StatusStore`
3. **Define typed options** in `domain/task/options.go`:
   - `WithTrackableType(t)` → `WithCondition("trackable_type", string(t))`
   - `WithTrackableID(id)` → `WithCondition("trackable_id", id)`
4. **Update callers** to use generic methods:
   - `FindByTrackable(ctx, type, id)` → `Find(ctx, WithTrackableType(type), WithTrackableID(id), repository.WithOrderAsc("created_at"))`
   - `DeleteByTrackable(ctx, type, id)` → `DeleteBy(ctx, WithTrackableType(type), WithTrackableID(id))`
   - `Count(ctx)` → `Count(ctx)` from embedding
5. **Keep as custom methods** (justified):
   - `Save(ctx, status) (Status, error)` — standard save
   - `LoadWithHierarchy(ctx, type, id)` — reconstructs parent-child relationships in domain logic after a `Find`; could potentially be split into a `Find` + domain function, but the hierarchy reconstruction depends on model-level `ParentID` which is not exposed in the domain type, so keeping this as a store method with an overridden Find is acceptable

### Resulting Interface

```go
type StatusStore interface {
    // From embedded Repository:
    // Find(ctx, ...options) ([]Status, error)
    // FindOne(ctx, ...options) (Status, error)
    // Exists(ctx, ...options) (bool, error)
    // Count(ctx, ...options) (int64, error)
    // DeleteBy(ctx, ...options) error

    Save(ctx context.Context, status Status) (Status, error)
    LoadWithHierarchy(ctx context.Context, trackableType TrackableType, trackableID int64) ([]Status, error)
}
```

---

## Finding 4: In-Memory Filtering in Queue.List

**File:** `application/service/queue.go` (lines 83-91)

`Queue.List()` fetches all tasks from `FindPending`, then filters by `Operation` in a Go loop. This should be pushed to the database using a `WithOperation` option.

### Action

After the TaskStore refactor, change `Queue.List` to pass `task.WithOperation(op)` as an option to `Find()`, eliminating the in-memory filtering loop.

---

## Implementation Order

1. **CommitIndexStore removal** — no dependencies, safe to delete
2. **StatusStore conversion** — mapper already conforms, straightforward embedding
3. **TaskStore conversion** — requires Task domain model restructuring first
4. **Queue.List optimization** — depends on TaskStore having WithOperation option

---

## Files Changed Per Step

### Step 1: Remove CommitIndexStore
- `domain/snippet/store.go` — remove interface
- `infrastructure/persistence/snippet_store.go` — remove implementation
- `infrastructure/persistence/mappers.go` — remove `CommitIndexMapper` if unused elsewhere
- Remove model type if only used by this store

### Step 2: Convert StatusStore
- `domain/task/store.go` — slim down `StatusStore` interface
- `domain/task/options.go` (new) — add `WithTrackableType`, `WithTrackableID`
- `infrastructure/persistence/task_store.go` — embed Repository, remove raw queries
- `application/handler/commit/rescan.go` — update `DeleteByTrackable` → `DeleteBy`
- `application/handler/commit/rescan_test.go` — update test calls
- `application/service/tracking.go` — no change (uses `LoadWithHierarchy` which stays)

### Step 3: Convert TaskStore
- `domain/task/task.go` — restructure to store payload bytes
- `domain/task/store.go` — slim down `TaskStore` interface
- `domain/task/options.go` — add `WithOperation`, `WithDedupKey`, `WithPriorityOrder`
- `infrastructure/persistence/mappers.go` — make `TaskMapper` infallible
- `infrastructure/persistence/task_store.go` — embed Repository, remove raw queries
- `application/service/queue.go` — update all store calls
- `application/service/worker.go` — update `Dequeue`/`Delete` calls
- `application/service/tracking.go` — update `CountPending` → `Count`
- Tests that reference the old interface methods

### Step 4: Queue.List Optimization
- `application/service/queue.go` — pass `WithOperation` option, remove in-memory loop
