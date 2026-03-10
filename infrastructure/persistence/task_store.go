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
	database.Repository[task.Task, TaskModel]
}

// NewTaskStore creates a new TaskStore.
func NewTaskStore(db database.Database) TaskStore {
	return TaskStore{
		Repository: database.NewRepository[task.Task, TaskModel](db, TaskMapper{}, "task"),
	}
}

// Get retrieves a task by ID.
func (s TaskStore) Get(ctx context.Context, id int64) (task.Task, error) {
	var model TaskModel
	result := s.DB(ctx).Where("id = ?", id).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return task.Task{}, fmt.Errorf("%w: task id %d", database.ErrNotFound, id)
		}
		return task.Task{}, fmt.Errorf("get task: %w", result.Error)
	}
	return s.Mapper().ToDomain(model), nil
}

// FindAll retrieves all tasks.
func (s TaskStore) FindAll(ctx context.Context) ([]task.Task, error) {
	var models []TaskModel
	result := s.DB(ctx).Order("priority DESC, created_at ASC").Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find all tasks: %w", result.Error)
	}

	tasks := make([]task.Task, len(models))
	for i, model := range models {
		tasks[i] = s.Mapper().ToDomain(model)
	}
	return tasks, nil
}

// FindPending retrieves pending tasks ordered by priority.
func (s TaskStore) FindPending(ctx context.Context, options ...repository.Option) ([]task.Task, error) {
	var models []TaskModel
	db := s.DB(ctx).Order("priority DESC, created_at ASC")
	db = database.ApplyOptions(db, options...)
	result := db.Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find pending tasks: %w", result.Error)
	}

	tasks := make([]task.Task, len(models))
	for i, model := range models {
		tasks[i] = s.Mapper().ToDomain(model)
	}
	return tasks, nil
}

// Save creates a new task or updates an existing one.
// Uses dedup_key for conflict resolution.
func (s TaskStore) Save(ctx context.Context, t task.Task) (task.Task, error) {
	model := s.Mapper().ToModel(t)

	result := s.DB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dedup_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"priority", "updated_at"}),
	}).Create(&model)

	if result.Error != nil {
		return task.Task{}, fmt.Errorf("save task: %w", result.Error)
	}

	return s.Mapper().ToDomain(model), nil
}

// SaveBulk creates or updates multiple tasks.
func (s TaskStore) SaveBulk(ctx context.Context, tasks []task.Task) ([]task.Task, error) {
	if len(tasks) == 0 {
		return []task.Task{}, nil
	}

	models := make([]TaskModel, len(tasks))
	for i, t := range tasks {
		models[i] = s.Mapper().ToModel(t)
	}

	result := s.DB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dedup_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"priority", "updated_at"}),
	}).Create(&models)

	if result.Error != nil {
		return nil, fmt.Errorf("save tasks bulk: %w", result.Error)
	}

	saved := make([]task.Task, len(models))
	for i, model := range models {
		saved[i] = s.Mapper().ToDomain(model)
	}
	return saved, nil
}

// Delete removes a task.
func (s TaskStore) Delete(ctx context.Context, t task.Task) error {
	result := s.DB(ctx).Delete(&TaskModel{}, t.ID())
	if result.Error != nil {
		return fmt.Errorf("delete task: %w", result.Error)
	}
	return nil
}

// DeleteAll removes all tasks.
func (s TaskStore) DeleteAll(ctx context.Context) error {
	result := s.DB(ctx).Where("1 = 1").Delete(&TaskModel{})
	if result.Error != nil {
		return fmt.Errorf("delete all tasks: %w", result.Error)
	}
	return nil
}

// CountPending returns the number of pending tasks.
func (s TaskStore) CountPending(ctx context.Context, options ...repository.Option) (int64, error) {
	var count int64
	db := database.ApplyConditions(s.DB(ctx).Model(&TaskModel{}), options...)
	result := db.Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count pending tasks: %w", result.Error)
	}
	return count, nil
}

// Exists checks if a task with the given ID exists.
func (s TaskStore) Exists(ctx context.Context, id int64) (bool, error) {
	var count int64
	result := s.DB(ctx).Model(&TaskModel{}).Where("id = ?", id).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check task exists: %w", result.Error)
	}
	return count > 0, nil
}

// Dequeue retrieves and removes the highest priority task.
func (s TaskStore) Dequeue(ctx context.Context) (task.Task, bool, error) {
	var model TaskModel

	err := s.DB(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Order("priority DESC, created_at ASC").First(&model)
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
		return task.Task{}, false, fmt.Errorf("dequeue task: %w", err)
	}

	if model.ID == 0 {
		return task.Task{}, false, nil
	}

	return s.Mapper().ToDomain(model), true, nil
}

// DequeueByOperation retrieves and removes the highest priority task of a specific operation type.
func (s TaskStore) DequeueByOperation(ctx context.Context, operation task.Operation) (task.Task, bool, error) {
	var model TaskModel

	err := s.DB(ctx).Transaction(func(tx *gorm.DB) error {
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

	return s.Mapper().ToDomain(model), true, nil
}
