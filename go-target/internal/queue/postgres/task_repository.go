package postgres

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/queue"
)

// TaskRepository implements queue.TaskRepository using PostgreSQL.
type TaskRepository struct {
	db     database.Database
	mapper TaskMapper
}

// NewTaskRepository creates a new PostgreSQL TaskRepository.
func NewTaskRepository(db database.Database) *TaskRepository {
	return &TaskRepository{
		db:     db,
		mapper: TaskMapper{},
	}
}

// Get retrieves a task by ID.
func (r *TaskRepository) Get(ctx context.Context, id int64) (queue.Task, error) {
	var entity TaskEntity
	result := r.db.Session(ctx).Where("id = ?", id).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return queue.Task{}, fmt.Errorf("%w: id %d", database.ErrNotFound, id)
		}
		return queue.Task{}, fmt.Errorf("get task: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Find retrieves tasks matching a query.
func (r *TaskRepository) Find(ctx context.Context, query database.Query) ([]queue.Task, error) {
	var entities []TaskEntity
	result := query.Apply(r.db.Session(ctx).Model(&TaskEntity{})).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find tasks: %w", result.Error)
	}

	tasks := make([]queue.Task, len(entities))
	for i, entity := range entities {
		tasks[i] = r.mapper.ToDomain(entity)
	}
	return tasks, nil
}

// Save creates a new task or updates an existing one.
// Uses dedup_key for conflict resolution.
func (r *TaskRepository) Save(ctx context.Context, task queue.Task) (queue.Task, error) {
	entity := r.mapper.ToDatabase(task)

	// Use upsert semantics based on dedup_key
	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dedup_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"priority", "updated_at"}),
	}).Create(&entity)

	if result.Error != nil {
		return queue.Task{}, fmt.Errorf("save task: %w", result.Error)
	}

	return r.mapper.ToDomain(entity), nil
}

// SaveBulk creates or updates multiple tasks.
func (r *TaskRepository) SaveBulk(ctx context.Context, tasks []queue.Task) ([]queue.Task, error) {
	if len(tasks) == 0 {
		return []queue.Task{}, nil
	}

	entities := make([]TaskEntity, len(tasks))
	for i, task := range tasks {
		entities[i] = r.mapper.ToDatabase(task)
	}

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "dedup_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"priority", "updated_at"}),
	}).Create(&entities)

	if result.Error != nil {
		return nil, fmt.Errorf("save tasks bulk: %w", result.Error)
	}

	savedTasks := make([]queue.Task, len(entities))
	for i, entity := range entities {
		savedTasks[i] = r.mapper.ToDomain(entity)
	}
	return savedTasks, nil
}

// Delete removes a task.
func (r *TaskRepository) Delete(ctx context.Context, task queue.Task) error {
	result := r.db.Session(ctx).Delete(&TaskEntity{}, task.ID())
	if result.Error != nil {
		return fmt.Errorf("delete task: %w", result.Error)
	}
	return nil
}

// DeleteByQuery removes tasks matching a query.
func (r *TaskRepository) DeleteByQuery(ctx context.Context, query database.Query) error {
	result := query.Apply(r.db.Session(ctx).Model(&TaskEntity{})).Delete(&TaskEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete tasks: %w", result.Error)
	}
	return nil
}

// Count returns the number of tasks matching a query.
func (r *TaskRepository) Count(ctx context.Context, query database.Query) (int64, error) {
	var count int64
	result := query.Apply(r.db.Session(ctx).Model(&TaskEntity{})).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count tasks: %w", result.Error)
	}
	return count, nil
}

// Exists checks if a task with the given ID exists.
func (r *TaskRepository) Exists(ctx context.Context, id int64) (bool, error) {
	var count int64
	result := r.db.Session(ctx).Model(&TaskEntity{}).Where("id = ?", id).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check task exists: %w", result.Error)
	}
	return count > 0, nil
}

// Dequeue retrieves and removes the highest priority task.
func (r *TaskRepository) Dequeue(ctx context.Context) (queue.Task, bool, error) {
	var entity TaskEntity

	// Use transaction to ensure atomicity
	err := r.db.Session(ctx).Transaction(func(tx *gorm.DB) error {
		// Find the highest priority task
		result := tx.Order("priority DESC, created_at ASC").First(&entity)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return nil // No tasks available
			}
			return result.Error
		}

		// Delete the task
		if err := tx.Delete(&entity).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return queue.Task{}, false, fmt.Errorf("dequeue task: %w", err)
	}

	if entity.ID == 0 {
		return queue.Task{}, false, nil
	}

	return r.mapper.ToDomain(entity), true, nil
}

// DequeueByOperation retrieves and removes the highest priority task of a specific operation type.
func (r *TaskRepository) DequeueByOperation(ctx context.Context, operation queue.TaskOperation) (queue.Task, bool, error) {
	var entity TaskEntity

	err := r.db.Session(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("type = ?", operation.String()).
			Order("priority DESC, created_at ASC").
			First(&entity)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return nil
			}
			return result.Error
		}

		if err := tx.Delete(&entity).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return queue.Task{}, false, fmt.Errorf("dequeue task by operation: %w", err)
	}

	if entity.ID == 0 {
		return queue.Task{}, false, nil
	}

	return r.mapper.ToDomain(entity), true, nil
}
