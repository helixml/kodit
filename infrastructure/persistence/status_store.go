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

// Get retrieves a task status by ID.
func (s StatusStore) Get(ctx context.Context, id string) (task.Status, error) {
	var model TaskStatusModel
	result := s.DB(ctx).Where("id = ?", id).First(&model)
	if result.Error != nil {
		return task.Status{}, fmt.Errorf("%w: status id %s", database.ErrNotFound, id)
	}
	return s.Mapper().ToDomain(model), nil
}

// FindByTrackable retrieves task statuses for a trackable entity.
func (s StatusStore) FindByTrackable(ctx context.Context, trackableType task.TrackableType, trackableID int64) ([]task.Status, error) {
	var models []TaskStatusModel
	result := s.DB(ctx).
		Where("trackable_type = ? AND trackable_id = ?", string(trackableType), trackableID).
		Order("created_at ASC").
		Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find statuses: %w", result.Error)
	}

	statuses := make([]task.Status, len(models))
	for i, model := range models {
		statuses[i] = s.Mapper().ToDomain(model)
	}
	return statuses, nil
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

// SaveBulk creates or updates multiple task statuses.
func (s StatusStore) SaveBulk(ctx context.Context, statuses []task.Status) ([]task.Status, error) {
	if len(statuses) == 0 {
		return []task.Status{}, nil
	}

	models := make([]TaskStatusModel, len(statuses))
	for i, status := range statuses {
		models[i] = s.Mapper().ToModel(status)
	}

	result := s.DB(ctx).Save(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("save statuses bulk: %w", result.Error)
	}

	saved := make([]task.Status, len(models))
	for i, model := range models {
		saved[i] = s.Mapper().ToDomain(model)
	}
	return saved, nil
}

// Delete removes a task status.
func (s StatusStore) Delete(ctx context.Context, status task.Status) error {
	result := s.DB(ctx).Delete(&TaskStatusModel{}, "id = ?", status.ID())
	if result.Error != nil {
		return fmt.Errorf("delete status: %w", result.Error)
	}
	return nil
}

// Count returns the total number of task statuses.
func (s StatusStore) Count(ctx context.Context) (int64, error) {
	return s.Repository.Count(ctx)
}

// DeleteByTrackable removes task statuses for a trackable entity.
func (s StatusStore) DeleteByTrackable(ctx context.Context, trackableType task.TrackableType, trackableID int64) error {
	result := s.DB(ctx).
		Where("trackable_type = ? AND trackable_id = ?", string(trackableType), trackableID).
		Delete(&TaskStatusModel{})
	if result.Error != nil {
		return fmt.Errorf("delete statuses: %w", result.Error)
	}
	return nil
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
