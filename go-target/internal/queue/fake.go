package queue

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/domain"
)

// FakeTaskRepository is an in-memory implementation of TaskRepository for testing.
type FakeTaskRepository struct {
	mu       sync.RWMutex
	tasks    map[int64]Task
	byDedup  map[string]int64
	sequence int64
}

// NewFakeTaskRepository creates a new fake task repository.
func NewFakeTaskRepository() *FakeTaskRepository {
	return &FakeTaskRepository{
		tasks:   make(map[int64]Task),
		byDedup: make(map[string]int64),
	}
}

// Get retrieves a task by ID.
func (r *FakeTaskRepository) Get(_ context.Context, id int64) (Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, ok := r.tasks[id]
	if !ok {
		return Task{}, database.ErrNotFound
	}
	return task, nil
}

// Find retrieves tasks matching a query.
func (r *FakeTaskRepository) Find(_ context.Context, query database.Query) ([]Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if r.matches(task, query) {
			result = append(result, task)
		}
	}

	// Apply ordering (multi-field sort)
	orders := query.Orders()
	if len(orders) > 0 {
		sortTasksMulti(result, orders)
	}

	// Apply offset and limit
	if offset := query.OffsetValue(); offset > 0 {
		if offset >= len(result) {
			return []Task{}, nil
		}
		result = result[offset:]
	}
	if limit := query.LimitValue(); limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result, nil
}

func (r *FakeTaskRepository) matches(task Task, query database.Query) bool {
	for _, filter := range query.Filters() {
		if !r.matchFilter(task, filter) {
			return false
		}
	}
	return true
}

func (r *FakeTaskRepository) matchFilter(task Task, filter database.Filter) bool {
	switch filter.Field() {
	case "type", "operation":
		val, ok := filter.Value().(string)
		if !ok {
			return false
		}
		switch filter.Operator() {
		case database.OpEqual:
			return task.Operation().String() == val
		case database.OpNotEqual:
			return task.Operation().String() != val
		}
	case "dedup_key":
		val, ok := filter.Value().(string)
		if !ok {
			return false
		}
		switch filter.Operator() {
		case database.OpEqual:
			return task.DedupKey() == val
		case database.OpNotEqual:
			return task.DedupKey() != val
		}
	}
	return true
}

func sortTasksMulti(tasks []Task, orders []database.OrderBy) {
	sort.Slice(tasks, func(i, j int) bool {
		for _, order := range orders {
			cmp := compareTaskField(tasks[i], tasks[j], order.Field())
			if cmp == 0 {
				continue // Equal on this field, check next
			}
			if order.Direction() == database.SortDesc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false // All fields equal
	})
}

func compareTaskField(a, b Task, field string) int {
	switch field {
	case "priority":
		if a.Priority() < b.Priority() {
			return -1
		}
		if a.Priority() > b.Priority() {
			return 1
		}
		return 0
	case "created_at":
		if a.CreatedAt().Before(b.CreatedAt()) {
			return -1
		}
		if a.CreatedAt().After(b.CreatedAt()) {
			return 1
		}
		return 0
	default:
		if a.ID() < b.ID() {
			return -1
		}
		if a.ID() > b.ID() {
			return 1
		}
		return 0
	}
}

// Save creates a new task or updates an existing one.
func (r *FakeTaskRepository) Save(_ context.Context, task Task) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Check for existing task with same dedup key
	if existingID, ok := r.byDedup[task.DedupKey()]; ok {
		existing := r.tasks[existingID]
		// Update the existing task with new values
		updated := NewTaskWithID(
			existingID,
			task.DedupKey(),
			task.Operation(),
			task.Priority(),
			task.Payload(),
			existing.CreatedAt(),
			now,
		)
		r.tasks[existingID] = updated
		return updated, nil
	}

	// Create new task
	r.sequence++
	id := r.sequence
	saved := task.WithID(id).WithTimestamps(now, now)
	r.tasks[id] = saved
	r.byDedup[task.DedupKey()] = id
	return saved, nil
}

// SaveBulk creates or updates multiple tasks.
func (r *FakeTaskRepository) SaveBulk(ctx context.Context, tasks []Task) ([]Task, error) {
	result := make([]Task, len(tasks))
	for i, task := range tasks {
		saved, err := r.Save(ctx, task)
		if err != nil {
			return nil, err
		}
		result[i] = saved
	}
	return result, nil
}

// Delete removes a task.
// Note: Does not return error if task doesn't exist (matches PostgreSQL behavior).
func (r *FakeTaskRepository) Delete(_ context.Context, task Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete without checking existence (idempotent like PostgreSQL)
	delete(r.byDedup, task.DedupKey())
	delete(r.tasks, task.ID())
	return nil
}

// DeleteByQuery removes tasks matching a query.
func (r *FakeTaskRepository) DeleteByQuery(ctx context.Context, query database.Query) error {
	tasks, err := r.Find(ctx, query)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, task := range tasks {
		delete(r.byDedup, task.DedupKey())
		delete(r.tasks, task.ID())
	}
	return nil
}

// Count returns the number of tasks matching a query.
func (r *FakeTaskRepository) Count(ctx context.Context, query database.Query) (int64, error) {
	tasks, err := r.Find(ctx, query)
	if err != nil {
		return 0, err
	}
	return int64(len(tasks)), nil
}

// Exists checks if a task with the given ID exists.
func (r *FakeTaskRepository) Exists(_ context.Context, id int64) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.tasks[id]
	return ok, nil
}

// Dequeue retrieves and removes the highest priority task.
func (r *FakeTaskRepository) Dequeue(_ context.Context) (Task, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.tasks) == 0 {
		return Task{}, false, nil
	}

	// Find highest priority task
	var highest Task
	var found bool
	for _, task := range r.tasks {
		if !found || task.Priority() > highest.Priority() {
			highest = task
			found = true
		}
	}

	// Remove it
	delete(r.byDedup, highest.DedupKey())
	delete(r.tasks, highest.ID())
	return highest, true, nil
}

// DequeueByOperation retrieves and removes the highest priority task of a type.
func (r *FakeTaskRepository) DequeueByOperation(_ context.Context, operation TaskOperation) (Task, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var highest Task
	var found bool
	for _, task := range r.tasks {
		if task.Operation() != operation {
			continue
		}
		if !found || task.Priority() > highest.Priority() {
			highest = task
			found = true
		}
	}

	if !found {
		return Task{}, false, nil
	}

	// Remove it
	delete(r.byDedup, highest.DedupKey())
	delete(r.tasks, highest.ID())
	return highest, true, nil
}

// All returns all tasks in the repository (test helper).
func (r *FakeTaskRepository) All() []Task {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		result = append(result, task)
	}
	return result
}

// FakeTaskStatusRepository is an in-memory implementation for testing.
type FakeTaskStatusRepository struct {
	mu       sync.RWMutex
	statuses map[string]TaskStatus
}

// NewFakeTaskStatusRepository creates a new fake task status repository.
func NewFakeTaskStatusRepository() *FakeTaskStatusRepository {
	return &FakeTaskStatusRepository{
		statuses: make(map[string]TaskStatus),
	}
}

// Get retrieves a task status by ID.
func (r *FakeTaskStatusRepository) Get(_ context.Context, id string) (TaskStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status, ok := r.statuses[id]
	if !ok {
		return TaskStatus{}, database.ErrNotFound
	}
	return status, nil
}

// Find retrieves task statuses matching a query.
func (r *FakeTaskStatusRepository) Find(_ context.Context, _ database.Query) ([]TaskStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]TaskStatus, 0, len(r.statuses))
	for _, status := range r.statuses {
		result = append(result, status)
	}
	return result, nil
}

// Save creates a new task status or updates an existing one.
func (r *FakeTaskStatusRepository) Save(_ context.Context, status TaskStatus) (TaskStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.statuses[status.ID()] = status
	return status, nil
}

// SaveBulk creates or updates multiple task statuses.
func (r *FakeTaskStatusRepository) SaveBulk(ctx context.Context, statuses []TaskStatus) ([]TaskStatus, error) {
	result := make([]TaskStatus, len(statuses))
	for i, status := range statuses {
		saved, err := r.Save(ctx, status)
		if err != nil {
			return nil, err
		}
		result[i] = saved
	}
	return result, nil
}

// Delete removes a task status.
func (r *FakeTaskStatusRepository) Delete(_ context.Context, status TaskStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[status.ID()]; !ok {
		return database.ErrNotFound
	}
	delete(r.statuses, status.ID())
	return nil
}

// DeleteByQuery removes task statuses matching a query.
func (r *FakeTaskStatusRepository) DeleteByQuery(ctx context.Context, query database.Query) error {
	statuses, err := r.Find(ctx, query)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, status := range statuses {
		delete(r.statuses, status.ID())
	}
	return nil
}

// Count returns the number of task statuses matching a query.
func (r *FakeTaskStatusRepository) Count(ctx context.Context, query database.Query) (int64, error) {
	statuses, err := r.Find(ctx, query)
	if err != nil {
		return 0, err
	}
	return int64(len(statuses)), nil
}

// LoadWithHierarchy loads all task statuses for a trackable entity.
func (r *FakeTaskStatusRepository) LoadWithHierarchy(
	_ context.Context,
	trackableType domain.TrackableType,
	trackableID int64,
) ([]TaskStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []TaskStatus
	for _, status := range r.statuses {
		if status.TrackableType() == trackableType && status.TrackableID() == trackableID {
			result = append(result, status)
		}
	}
	return result, nil
}

// FakeHandler is a test handler that records execution.
type FakeHandler struct {
	mu       sync.Mutex
	Calls    []map[string]any
	ReturnFn func(payload map[string]any) error
}

// NewFakeHandler creates a new fake handler.
func NewFakeHandler() *FakeHandler {
	return &FakeHandler{
		Calls: make([]map[string]any, 0),
	}
}

// Execute records the call and returns the configured result.
func (h *FakeHandler) Execute(_ context.Context, payload map[string]any) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.Calls = append(h.Calls, payload)
	if h.ReturnFn != nil {
		return h.ReturnFn(payload)
	}
	return nil
}

// CallCount returns the number of times Execute was called.
func (h *FakeHandler) CallCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.Calls)
}

// LastCall returns the payload from the last call.
func (h *FakeHandler) LastCall() map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.Calls) == 0 {
		return nil
	}
	return h.Calls[len(h.Calls)-1]
}
