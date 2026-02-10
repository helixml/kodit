package database

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ErrNotFound indicates the requested entity was not found.
var ErrNotFound = errors.New("entity not found")

// EntityMapper defines the interface for mapping between domain and database entities.
type EntityMapper[D any, E any] interface {
	ToDomain(entity E) D
	ToDatabase(domain D) E
}

// Repository provides generic CRUD operations for database entities.
type Repository[D any, E any] struct {
	db     Database
	mapper EntityMapper[D, E]
	table  string
}

// NewRepository creates a new Repository.
func NewRepository[D any, E any](db Database, mapper EntityMapper[D, E], table string) Repository[D, E] {
	return Repository[D, E]{
		db:     db,
		mapper: mapper,
		table:  table,
	}
}

// Get retrieves an entity by ID.
func (r Repository[D, E]) Get(ctx context.Context, id int64) (D, error) {
	var entity E
	result := r.db.Session(ctx).Table(r.table).Where("id = ?", id).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			var zero D
			return zero, fmt.Errorf("%w: id %d", ErrNotFound, id)
		}
		var zero D
		return zero, fmt.Errorf("get entity: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Find retrieves entities matching a query.
func (r Repository[D, E]) Find(ctx context.Context, query Query) ([]D, error) {
	var entities []E
	result := query.Apply(r.db.Session(ctx).Table(r.table)).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find entities: %w", result.Error)
	}

	domains := make([]D, len(entities))
	for i, entity := range entities {
		domains[i] = r.mapper.ToDomain(entity)
	}
	return domains, nil
}

// FindAll retrieves all entities.
func (r Repository[D, E]) FindAll(ctx context.Context) ([]D, error) {
	return r.Find(ctx, NewQuery())
}

// Save creates or updates an entity.
func (r Repository[D, E]) Save(ctx context.Context, domain D) (D, error) {
	entity := r.mapper.ToDatabase(domain)
	result := r.db.Session(ctx).Table(r.table).Save(&entity)
	if result.Error != nil {
		var zero D
		return zero, fmt.Errorf("save entity: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Create creates a new entity.
func (r Repository[D, E]) Create(ctx context.Context, domain D) (D, error) {
	entity := r.mapper.ToDatabase(domain)
	result := r.db.Session(ctx).Table(r.table).Create(&entity)
	if result.Error != nil {
		var zero D
		return zero, fmt.Errorf("create entity: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Update updates an existing entity.
func (r Repository[D, E]) Update(ctx context.Context, domain D) error {
	entity := r.mapper.ToDatabase(domain)
	result := r.db.Session(ctx).Table(r.table).Save(&entity)
	if result.Error != nil {
		return fmt.Errorf("update entity: %w", result.Error)
	}
	return nil
}

// Delete removes an entity.
func (r Repository[D, E]) Delete(ctx context.Context, domain D) error {
	entity := r.mapper.ToDatabase(domain)
	result := r.db.Session(ctx).Table(r.table).Delete(&entity)
	if result.Error != nil {
		return fmt.Errorf("delete entity: %w", result.Error)
	}
	return nil
}

// DeleteByID removes an entity by ID.
func (r Repository[D, E]) DeleteByID(ctx context.Context, id int64) error {
	var entity E
	result := r.db.Session(ctx).Table(r.table).Where("id = ?", id).Delete(&entity)
	if result.Error != nil {
		return fmt.Errorf("delete entity: %w", result.Error)
	}
	return nil
}

// Count returns the count of entities matching a query.
func (r Repository[D, E]) Count(ctx context.Context, query Query) (int64, error) {
	var count int64
	result := query.Apply(r.db.Session(ctx).Table(r.table)).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count entities: %w", result.Error)
	}
	return count, nil
}

// Exists checks if any entity matches the query.
func (r Repository[D, E]) Exists(ctx context.Context, query Query) (bool, error) {
	count, err := r.Count(ctx, query)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsByID checks if an entity with the given ID exists.
func (r Repository[D, E]) ExistsByID(ctx context.Context, id int64) (bool, error) {
	return r.Exists(ctx, NewQuery().Equal("id", id))
}

// FindOne retrieves a single entity matching a query.
func (r Repository[D, E]) FindOne(ctx context.Context, query Query) (D, error) {
	var entity E
	result := query.Apply(r.db.Session(ctx).Table(r.table)).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			var zero D
			return zero, ErrNotFound
		}
		var zero D
		return zero, fmt.Errorf("find one: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Session returns the underlying GORM session for custom queries.
func (r Repository[D, E]) Session(ctx context.Context) *gorm.DB {
	return r.db.Session(ctx).Table(r.table)
}
