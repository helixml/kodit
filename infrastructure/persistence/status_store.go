package persistence

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/internal/database"
)

// StatusStore implements task.StatusStore using GORM.
type StatusStore struct {
	database.Repository[task.Status, TaskStatusModel]
}

// NewStatusStore creates a new StatusStore.
func NewStatusStore(db database.Database) StatusStore {
	return StatusStore{
		Repository: database.NewRepository[task.Status, TaskStatusModel](db, TaskStatusMapper{}, "status"),
	}
}

// Save creates a new task status or updates an existing one.
func (s StatusStore) Save(ctx context.Context, status task.Status) (task.Status, error) {
	model := s.Mapper().ToModel(status)

	result := s.DB(ctx).Save(&model)
	if result.Error != nil {
		return task.Status{}, fmt.Errorf("save status: %w", result.Error)
	}

	return s.Mapper().ToDomain(model), nil
}

// LoadWithHierarchy loads all task statuses for a trackable entity
// with their parent-child relationships reconstructed.
func (s StatusStore) LoadWithHierarchy(ctx context.Context, trackableType task.TrackableType, trackableID int64) ([]task.Status, error) {
	var models []TaskStatusModel
	result := s.DB(ctx).
		Where("trackable_type = ? AND trackable_id = ?", string(trackableType), trackableID).
		Order("created_at ASC").
		Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("load with hierarchy: %w", result.Error)
	}

	statusMap := make(map[string]*task.Status)
	for _, model := range models {
		status := s.Mapper().ToDomain(model)
		statusMap[model.ID] = &status
	}

	statuses := make([]task.Status, 0, len(models))
	for _, model := range models {
		var parent *task.Status
		if model.ParentID != nil {
			if p, ok := statusMap[*model.ParentID]; ok {
				parent = p
			}
		}

		var tID int64
		var tType task.TrackableType
		if model.TrackableID != nil {
			tID = *model.TrackableID
		}
		if model.TrackableType != nil {
			tType = task.TrackableType(*model.TrackableType)
		}

		status := task.NewStatusFull(
			model.ID,
			task.ReportingState(model.State),
			task.Operation(model.Operation),
			model.Message,
			model.CreatedAt,
			model.UpdatedAt,
			model.Total,
			model.Current,
			model.Error,
			parent,
			tID,
			tType,
		)
		statuses = append(statuses, status)
	}

	return statuses, nil
}
