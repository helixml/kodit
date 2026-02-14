package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TaskStore implements task.TaskStore using GORM.
type TaskStore struct {
	db     database.Database
	mapper TaskMapper
}

// NewTaskStore creates a new TaskStore.
func NewTaskStore(db database.Database) TaskStore {
	return TaskStore{
		db:     db,
		mapper: TaskMapper{},
	}
}

// Get retrieves a task by ID.
func (s TaskStore) Get(ctx context.Context, id int64) (task.Task, error) {
	var model TaskModel
	result := s.db.Session(ctx).Where("id = ?", id).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return task.Task{}, fmt.Errorf("%w: task id %d", database.ErrNotFound, id)
		}
		return task.Task{}, fmt.Errorf("get task: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// FindAll retrieves all tasks.
func (s TaskStore) FindAll(ctx context.Context) ([]task.Task, error) {
	var models []TaskModel
	result := s.db.Session(ctx).Order("priority DESC, created_at ASC").Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find all tasks: %w", result.Error)
	}

	tasks := make([]task.Task, len(models))
	for i, model := range models {
		tasks[i] = s.mapper.ToDomain(model)
	}
	return tasks, nil
}

// FindPending retrieves pending tasks ordered by priority.
func (s TaskStore) FindPending(ctx context.Context, options ...repository.Option) ([]task.Task, error) {
	var models []TaskModel
	db := s.db.Session(ctx).Order("priority DESC, created_at ASC")
	db = database.ApplyOptions(db, options...)
	result := db.Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find pending tasks: %w", result.Error)
	}

	tasks := make([]task.Task, len(models))
	for i, model := range models {
		tasks[i] = s.mapper.ToDomain(model)
	}
	return tasks, nil
}

// Save creates a new task or updates an existing one.
// Uses dedup_key for conflict resolution.
func (s TaskStore) Save(ctx context.Context, t task.Task) (task.Task, error) {
	model := s.mapper.ToModel(t)

	result := s.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dedup_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"priority", "updated_at"}),
	}).Create(&model)

	if result.Error != nil {
		return task.Task{}, fmt.Errorf("save task: %w", result.Error)
	}

	return s.mapper.ToDomain(model), nil
}

// SaveBulk creates or updates multiple tasks.
func (s TaskStore) SaveBulk(ctx context.Context, tasks []task.Task) ([]task.Task, error) {
	if len(tasks) == 0 {
		return []task.Task{}, nil
	}

	models := make([]TaskModel, len(tasks))
	for i, t := range tasks {
		models[i] = s.mapper.ToModel(t)
	}

	result := s.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dedup_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"priority", "updated_at"}),
	}).Create(&models)

	if result.Error != nil {
		return nil, fmt.Errorf("save tasks bulk: %w", result.Error)
	}

	saved := make([]task.Task, len(models))
	for i, model := range models {
		saved[i] = s.mapper.ToDomain(model)
	}
	return saved, nil
}

// Delete removes a task.
func (s TaskStore) Delete(ctx context.Context, t task.Task) error {
	result := s.db.Session(ctx).Delete(&TaskModel{}, t.ID())
	if result.Error != nil {
		return fmt.Errorf("delete task: %w", result.Error)
	}
	return nil
}

// DeleteAll removes all tasks.
func (s TaskStore) DeleteAll(ctx context.Context) error {
	result := s.db.Session(ctx).Where("1 = 1").Delete(&TaskModel{})
	if result.Error != nil {
		return fmt.Errorf("delete all tasks: %w", result.Error)
	}
	return nil
}

// CountPending returns the number of pending tasks.
func (s TaskStore) CountPending(ctx context.Context, options ...repository.Option) (int64, error) {
	var count int64
	db := database.ApplyConditions(s.db.Session(ctx).Model(&TaskModel{}), options...)
	result := db.Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count pending tasks: %w", result.Error)
	}
	return count, nil
}

// Exists checks if a task with the given ID exists.
func (s TaskStore) Exists(ctx context.Context, id int64) (bool, error) {
	var count int64
	result := s.db.Session(ctx).Model(&TaskModel{}).Where("id = ?", id).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check task exists: %w", result.Error)
	}
	return count > 0, nil
}

// Dequeue retrieves and removes the highest priority task.
func (s TaskStore) Dequeue(ctx context.Context) (task.Task, bool, error) {
	var model TaskModel

	err := s.db.Session(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Order("priority DESC, created_at ASC").First(&model)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return nil // No tasks available
			}
			return result.Error
		}

		if err := tx.Delete(&model).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return task.Task{}, false, fmt.Errorf("dequeue task: %w", err)
	}

	if model.ID == 0 {
		return task.Task{}, false, nil
	}

	return s.mapper.ToDomain(model), true, nil
}

// DequeueByOperation retrieves and removes the highest priority task of a specific operation type.
func (s TaskStore) DequeueByOperation(ctx context.Context, operation task.Operation) (task.Task, bool, error) {
	var model TaskModel

	err := s.db.Session(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("type = ?", operation.String()).
			Order("priority DESC, created_at ASC").
			First(&model)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return nil
			}
			return result.Error
		}

		if err := tx.Delete(&model).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return task.Task{}, false, fmt.Errorf("dequeue task by operation: %w", err)
	}

	if model.ID == 0 {
		return task.Task{}, false, nil
	}

	return s.mapper.ToDomain(model), true, nil
}

// StatusStore implements task.StatusStore using GORM.
type StatusStore struct {
	db     database.Database
	mapper TaskStatusMapper
}

// NewStatusStore creates a new StatusStore.
func NewStatusStore(db database.Database) StatusStore {
	return StatusStore{
		db:     db,
		mapper: TaskStatusMapper{},
	}
}

// Get retrieves a task status by ID.
func (s StatusStore) Get(ctx context.Context, id string) (task.Status, error) {
	var model TaskStatusModel
	result := s.db.Session(ctx).Where("id = ?", id).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return task.Status{}, fmt.Errorf("%w: status id %s", database.ErrNotFound, id)
		}
		return task.Status{}, fmt.Errorf("get status: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// FindByTrackable retrieves task statuses for a trackable entity.
func (s StatusStore) FindByTrackable(ctx context.Context, trackableType task.TrackableType, trackableID int64) ([]task.Status, error) {
	var models []TaskStatusModel
	result := s.db.Session(ctx).
		Where("trackable_type = ? AND trackable_id = ?", string(trackableType), trackableID).
		Order("created_at ASC").
		Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find statuses: %w", result.Error)
	}

	statuses := make([]task.Status, len(models))
	for i, model := range models {
		statuses[i] = s.mapper.ToDomain(model)
	}
	return statuses, nil
}

// Save creates a new task status or updates an existing one.
func (s StatusStore) Save(ctx context.Context, status task.Status) (task.Status, error) {
	model := s.mapper.ToModel(status)

	result := s.db.Session(ctx).Save(&model)
	if result.Error != nil {
		return task.Status{}, fmt.Errorf("save status: %w", result.Error)
	}

	return s.mapper.ToDomain(model), nil
}

// SaveBulk creates or updates multiple task statuses.
func (s StatusStore) SaveBulk(ctx context.Context, statuses []task.Status) ([]task.Status, error) {
	if len(statuses) == 0 {
		return []task.Status{}, nil
	}

	models := make([]TaskStatusModel, len(statuses))
	for i, status := range statuses {
		models[i] = s.mapper.ToModel(status)
	}

	result := s.db.Session(ctx).Save(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("save statuses bulk: %w", result.Error)
	}

	saved := make([]task.Status, len(models))
	for i, model := range models {
		saved[i] = s.mapper.ToDomain(model)
	}
	return saved, nil
}

// Delete removes a task status.
func (s StatusStore) Delete(ctx context.Context, status task.Status) error {
	result := s.db.Session(ctx).Delete(&TaskStatusModel{}, "id = ?", status.ID())
	if result.Error != nil {
		return fmt.Errorf("delete status: %w", result.Error)
	}
	return nil
}

// DeleteByTrackable removes task statuses for a trackable entity.
func (s StatusStore) DeleteByTrackable(ctx context.Context, trackableType task.TrackableType, trackableID int64) error {
	result := s.db.Session(ctx).
		Where("trackable_type = ? AND trackable_id = ?", string(trackableType), trackableID).
		Delete(&TaskStatusModel{})
	if result.Error != nil {
		return fmt.Errorf("delete statuses: %w", result.Error)
	}
	return nil
}

// Count returns the total number of task statuses.
func (s StatusStore) Count(ctx context.Context) (int64, error) {
	var count int64
	result := s.db.Session(ctx).Model(&TaskStatusModel{}).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count statuses: %w", result.Error)
	}
	return count, nil
}

// LoadWithHierarchy loads all task statuses for a trackable entity
// with their parent-child relationships reconstructed.
func (s StatusStore) LoadWithHierarchy(ctx context.Context, trackableType task.TrackableType, trackableID int64) ([]task.Status, error) {
	var models []TaskStatusModel
	result := s.db.Session(ctx).
		Where("trackable_type = ? AND trackable_id = ?", string(trackableType), trackableID).
		Order("created_at ASC").
		Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("load with hierarchy: %w", result.Error)
	}

	// First pass: build a map of statuses by ID (without parent links)
	statusMap := make(map[string]*task.Status)
	for _, model := range models {
		status := s.mapper.ToDomain(model)
		statusMap[model.ID] = &status
	}

	// Second pass: reconstruct parent links using NewStatusFull
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
