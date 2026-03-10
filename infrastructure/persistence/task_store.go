package persistence

import (
	"context"
	"errors"
	"fmt"

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

// Delete removes a task.
func (s TaskStore) Delete(ctx context.Context, t task.Task) error {
	result := s.DB(ctx).Delete(&TaskModel{}, t.ID())
	if result.Error != nil {
		return fmt.Errorf("delete task: %w", result.Error)
	}
	return nil
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
