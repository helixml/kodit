package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
	"gorm.io/gorm"
)

// ErrNotFound indicates the requested entity was not found.
var ErrNotFound = errors.New("entity not found")

// EntityMapper defines the interface for mapping between domain and database model types.
type EntityMapper[D any, E any] interface {
	ToDomain(entity E) D
	ToModel(domain D) E
}

// Repository provides generic persistence operations for database entities
// using repository.Option-based queries.
type Repository[D any, E any] struct {
	db     Database
	mapper EntityMapper[D, E]
	label  string
}

// NewRepository creates a new Repository.
func NewRepository[D any, E any](db Database, mapper EntityMapper[D, E], label string) Repository[D, E] {
	return Repository[D, E]{
		db:     db,
		mapper: mapper,
		label:  label,
	}
}

// Find retrieves entities matching the given options.
func (r Repository[D, E]) Find(ctx context.Context, options ...repository.Option) ([]D, error) {
	var entities []E
	db := ApplyOptions(r.db.Session(ctx).Model(new(E)), options...)
	result := db.Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find %s: %w", r.label, result.Error)
	}

	domains := make([]D, len(entities))
	for i, entity := range entities {
		domains[i] = r.mapper.ToDomain(entity)
	}
	return domains, nil
}

// FindOne retrieves a single entity matching the given options.
func (r Repository[D, E]) FindOne(ctx context.Context, options ...repository.Option) (D, error) {
	var entity E
	db := ApplyOptions(r.db.Session(ctx), options...)
	result := db.First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			var zero D
			return zero, fmt.Errorf("%w: %s", ErrNotFound, r.label)
		}
		var zero D
		return zero, fmt.Errorf("find one %s: %w", r.label, result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Exists checks if any entity matches the given options.
func (r Repository[D, E]) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	var count int64
	db := ApplyOptions(r.db.Session(ctx).Model(new(E)), options...)
	result := db.Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check %s exists: %w", r.label, result.Error)
	}
	return count > 0, nil
}

// DeleteBy removes entities matching the given options.
func (r Repository[D, E]) DeleteBy(ctx context.Context, options ...repository.Option) error {
	db := ApplyOptions(r.db.Session(ctx), options...)
	result := db.Delete(new(E))
	if result.Error != nil {
		return fmt.Errorf("delete %s: %w", r.label, result.Error)
	}
	return nil
}

// DB returns the underlying GORM session for custom queries.
func (r Repository[D, E]) DB(ctx context.Context) *gorm.DB {
	return r.db.Session(ctx)
}

// Mapper returns the entity mapper for external use.
func (r Repository[D, E]) Mapper() EntityMapper[D, E] {
	return r.mapper
}
