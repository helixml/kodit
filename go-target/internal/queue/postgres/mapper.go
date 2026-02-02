package postgres

import (
	"encoding/json"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/queue"
)

// TaskMapper maps between Task domain and database entities.
type TaskMapper struct{}

// ToDomain converts a database entity to a domain entity.
func (TaskMapper) ToDomain(entity TaskEntity) queue.Task {
	var payload map[string]any
	if len(entity.Payload) > 0 {
		_ = json.Unmarshal(entity.Payload, &payload)
	}
	if payload == nil {
		payload = make(map[string]any)
	}

	return queue.NewTaskWithID(
		entity.ID,
		entity.DedupKey,
		queue.TaskOperation(entity.Type),
		entity.Priority,
		payload,
		entity.CreatedAt,
		entity.UpdatedAt,
	)
}

// ToDatabase converts a domain entity to a database entity.
func (TaskMapper) ToDatabase(task queue.Task) TaskEntity {
	payloadJSON, _ := json.Marshal(task.Payload())

	return TaskEntity{
		ID:        task.ID(),
		DedupKey:  task.DedupKey(),
		Type:      task.Operation().String(),
		Payload:   payloadJSON,
		Priority:  task.Priority(),
		CreatedAt: task.CreatedAt(),
		UpdatedAt: task.UpdatedAt(),
	}
}

// TaskStatusMapper maps between TaskStatus domain and database entities.
type TaskStatusMapper struct{}

// ToDomain converts a database entity to a domain entity (without parent resolution).
func (TaskStatusMapper) ToDomain(entity TaskStatusEntity) queue.TaskStatus {
	var trackableID int64
	var trackableType domain.TrackableType

	if entity.TrackableID != nil {
		trackableID = *entity.TrackableID
	}
	if entity.TrackableType != nil {
		trackableType = domain.TrackableType(*entity.TrackableType)
	}

	return queue.NewTaskStatusFull(
		entity.ID,
		domain.ReportingState(entity.State),
		queue.TaskOperation(entity.Operation),
		entity.Message,
		entity.CreatedAt,
		entity.UpdatedAt,
		entity.Total,
		entity.Current,
		entity.Error,
		nil, // Parent is resolved separately
		trackableID,
		trackableType,
	)
}

// ToDomainWithHierarchy converts database entities to domain entities with parent relationships.
func (m TaskStatusMapper) ToDomainWithHierarchy(entities []TaskStatusEntity) []queue.TaskStatus {
	// Build a map of ID to domain status for parent resolution
	statusMap := make(map[string]queue.TaskStatus, len(entities))
	parentMap := make(map[string]string) // child ID -> parent ID

	// First pass: convert all entities and record parent relationships
	for _, entity := range entities {
		statusMap[entity.ID] = m.ToDomain(entity)
		if entity.ParentID != nil && *entity.ParentID != "" {
			parentMap[entity.ID] = *entity.ParentID
		}
	}

	// Second pass: resolve parent relationships
	// Note: We can't set the parent directly on the immutable struct,
	// so we need to return statuses in order where parents come first
	result := make([]queue.TaskStatus, 0, len(entities))

	// Add statuses without parents first
	for _, entity := range entities {
		if entity.ParentID == nil || *entity.ParentID == "" {
			result = append(result, statusMap[entity.ID])
		}
	}

	// Add statuses with parents
	for _, entity := range entities {
		if entity.ParentID != nil && *entity.ParentID != "" {
			result = append(result, statusMap[entity.ID])
		}
	}

	return result
}

// ToDatabase converts a domain entity to a database entity.
func (TaskStatusMapper) ToDatabase(status queue.TaskStatus) TaskStatusEntity {
	entity := TaskStatusEntity{
		ID:        status.ID(),
		CreatedAt: status.CreatedAt(),
		UpdatedAt: status.UpdatedAt(),
		Operation: status.Operation().String(),
		Message:   status.Message(),
		State:     string(status.State()),
		Error:     status.Error(),
		Total:     status.Total(),
		Current:   status.Current(),
	}

	if status.TrackableID() != 0 {
		id := status.TrackableID()
		entity.TrackableID = &id
	}

	if status.TrackableType() != "" {
		t := string(status.TrackableType())
		entity.TrackableType = &t
	}

	if status.Parent() != nil {
		parentID := status.Parent().ID()
		entity.ParentID = &parentID
	}

	return entity
}
