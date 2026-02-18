// Package task provides task queue domain types for async work processing.
package task

import (
	"encoding/json"
	"fmt"
	"maps"
	"time"
)

// Priority represents task queue priority levels.
// Values are spaced far apart to ensure batch offsets (up to ~150
// for 15 tasks) never cause a lower priority level to exceed a higher one.
type Priority int

// Priority values.
const (
	PriorityBackground    Priority = 1000
	PriorityNormal        Priority = 2000
	PriorityUserInitiated Priority = 5000
	PriorityCritical      Priority = 10000
)

// Task represents an item in the queue waiting to be processed.
// If the item exists, it is in the queue and waiting to be processed.
// There is no status associated - existence implies pending.
type Task struct {
	id        int64
	dedupKey  string
	operation Operation
	priority  int
	payload   map[string]any
	createdAt time.Time
	updatedAt time.Time
}

// NewTask creates a new Task with the given operation, priority, and payload.
// The dedup key is generated automatically from the operation and payload.
func NewTask(operation Operation, priority int, payload map[string]any) Task {
	p := copyPayload(payload)
	return Task{
		dedupKey:  createDedupKey(operation, p),
		operation: operation,
		priority:  priority,
		payload:   p,
	}
}

// NewTaskWithID creates a Task with all fields (used by repository).
func NewTaskWithID(
	id int64,
	dedupKey string,
	operation Operation,
	priority int,
	payload map[string]any,
	createdAt, updatedAt time.Time,
) Task {
	return Task{
		id:        id,
		dedupKey:  dedupKey,
		operation: operation,
		priority:  priority,
		payload:   copyPayload(payload),
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// ID returns the task ID.
func (t Task) ID() int64 { return t.id }

// DedupKey returns the deduplication key.
func (t Task) DedupKey() string { return t.dedupKey }

// Operation returns the task operation.
func (t Task) Operation() Operation { return t.operation }

// Priority returns the task priority.
func (t Task) Priority() int { return t.priority }

// Payload returns a copy of the task payload.
func (t Task) Payload() map[string]any {
	return copyPayload(t.payload)
}

// CreatedAt returns when the task was created.
func (t Task) CreatedAt() time.Time { return t.createdAt }

// UpdatedAt returns when the task was last updated.
func (t Task) UpdatedAt() time.Time { return t.updatedAt }

// WithID returns a copy of the task with the given ID.
func (t Task) WithID(id int64) Task {
	t.id = id
	return t
}

// WithTimestamps returns a copy of the task with the given timestamps.
func (t Task) WithTimestamps(createdAt, updatedAt time.Time) Task {
	t.createdAt = createdAt
	t.updatedAt = updatedAt
	return t
}

// PayloadJSON returns the payload as JSON bytes.
func (t Task) PayloadJSON() ([]byte, error) {
	return json.Marshal(t.payload)
}

// createDedupKey creates a unique key for deduplication.
// Format: "{operation}:{first_payload_value}"
func createDedupKey(operation Operation, payload map[string]any) string {
	// Get the first value from payload
	var firstVal any
	for _, v := range payload {
		firstVal = v
		break
	}
	return fmt.Sprintf("%s:%v", operation, firstVal)
}

// copyPayload creates a shallow copy of the payload map.
func copyPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return make(map[string]any)
	}
	result := make(map[string]any, len(payload))
	maps.Copy(result, payload)
	return result
}
