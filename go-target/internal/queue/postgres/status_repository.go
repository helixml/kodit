package postgres

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/queue"
)

// TaskStatusRepository implements queue.TaskStatusRepository using PostgreSQL.
type TaskStatusRepository struct {
	db     database.Database
	mapper TaskStatusMapper
}

// NewTaskStatusRepository creates a new PostgreSQL TaskStatusRepository.
func NewTaskStatusRepository(db database.Database) *TaskStatusRepository {
	return &TaskStatusRepository{
		db:     db,
		mapper: TaskStatusMapper{},
	}
}

// Get retrieves a task status by ID.
func (r *TaskStatusRepository) Get(ctx context.Context, id string) (queue.TaskStatus, error) {
	var entity TaskStatusEntity
	result := r.db.Session(ctx).Where("id = ?", id).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return queue.TaskStatus{}, fmt.Errorf("%w: id %s", database.ErrNotFound, id)
		}
		return queue.TaskStatus{}, fmt.Errorf("get task status: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Find retrieves task statuses matching a query.
func (r *TaskStatusRepository) Find(ctx context.Context, query database.Query) ([]queue.TaskStatus, error) {
	var entities []TaskStatusEntity
	result := query.Apply(r.db.Session(ctx).Model(&TaskStatusEntity{})).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find task statuses: %w", result.Error)
	}

	statuses := make([]queue.TaskStatus, len(entities))
	for i, entity := range entities {
		statuses[i] = r.mapper.ToDomain(entity)
	}
	return statuses, nil
}

// Save creates a new task status or updates an existing one.
// If the status has a parent, the parent chain is saved first.
func (r *TaskStatusRepository) Save(ctx context.Context, status queue.TaskStatus) (queue.TaskStatus, error) {
	// Collect parent chain (parents first, current last)
	var chain []queue.TaskStatus
	current := &status
	for current != nil {
		chain = append([]queue.TaskStatus{*current}, chain...)
		current = current.Parent()
	}

	// Save in order (parents first)
	var lastSaved queue.TaskStatus
	for _, s := range chain {
		entity := r.mapper.ToDatabase(s)
		result := r.db.Session(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"state", "message", "error", "total", "current", "updated_at"}),
		}).Create(&entity)

		if result.Error != nil {
			return queue.TaskStatus{}, fmt.Errorf("save task status: %w", result.Error)
		}
		lastSaved = r.mapper.ToDomain(entity)
	}

	return lastSaved, nil
}

// SaveBulk creates or updates multiple task statuses.
func (r *TaskStatusRepository) SaveBulk(ctx context.Context, statuses []queue.TaskStatus) ([]queue.TaskStatus, error) {
	if len(statuses) == 0 {
		return []queue.TaskStatus{}, nil
	}

	entities := make([]TaskStatusEntity, len(statuses))
	for i, status := range statuses {
		entities[i] = r.mapper.ToDatabase(status)
	}

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"state", "message", "error", "total", "current", "updated_at"}),
	}).Create(&entities)

	if result.Error != nil {
		return nil, fmt.Errorf("save task statuses bulk: %w", result.Error)
	}

	savedStatuses := make([]queue.TaskStatus, len(entities))
	for i, entity := range entities {
		savedStatuses[i] = r.mapper.ToDomain(entity)
	}
	return savedStatuses, nil
}

// Delete removes a task status.
func (r *TaskStatusRepository) Delete(ctx context.Context, status queue.TaskStatus) error {
	result := r.db.Session(ctx).Delete(&TaskStatusEntity{}, "id = ?", status.ID())
	if result.Error != nil {
		return fmt.Errorf("delete task status: %w", result.Error)
	}
	return nil
}

// DeleteByQuery removes task statuses matching a query.
func (r *TaskStatusRepository) DeleteByQuery(ctx context.Context, query database.Query) error {
	result := query.Apply(r.db.Session(ctx).Model(&TaskStatusEntity{})).Delete(&TaskStatusEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete task statuses: %w", result.Error)
	}
	return nil
}

// Count returns the number of task statuses matching a query.
func (r *TaskStatusRepository) Count(ctx context.Context, query database.Query) (int64, error) {
	var count int64
	result := query.Apply(r.db.Session(ctx).Model(&TaskStatusEntity{})).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count task statuses: %w", result.Error)
	}
	return count, nil
}

// LoadWithHierarchy loads all task statuses for a trackable entity with hierarchy.
func (r *TaskStatusRepository) LoadWithHierarchy(
	ctx context.Context,
	trackableType domain.TrackableType,
	trackableID int64,
) ([]queue.TaskStatus, error) {
	var entities []TaskStatusEntity
	result := r.db.Session(ctx).
		Where("trackable_type = ? AND trackable_id = ?", string(trackableType), trackableID).
		Find(&entities)

	if result.Error != nil {
		return nil, fmt.Errorf("load task statuses with hierarchy: %w", result.Error)
	}

	return r.mapper.ToDomainWithHierarchy(entities), nil
}
