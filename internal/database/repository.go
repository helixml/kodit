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
	db        Database
	mapper    EntityMapper[D, E]
	label     string
	tableName string
}

// NewRepository creates a new Repository.
func NewRepository[D any, E any](db Database, mapper EntityMapper[D, E], label string) Repository[D, E] {
	return Repository[D, E]{
		db:     db,
		mapper: mapper,
		label:  label,
	}
}

// NewRepositoryForTable creates a Repository that targets a specific table name.
// GORM caches schemas by type, so dynamic TableName() methods on entities do
// not work when the same struct maps to multiple tables. This constructor sets
// a tableName that is applied via .Table() after .Model() in every operation,
// which GORM respects because TableExpr prevents Parse() from overriding
// Statement.Table.
func NewRepositoryForTable[D any, E any](db Database, mapper EntityMapper[D, E], label string, tableName string) Repository[D, E] {
	return Repository[D, E]{
		db:        db,
		mapper:    mapper,
		label:     label,
		tableName: tableName,
	}
}

// Table returns the dynamic table name, or empty if not set.
func (r Repository[D, E]) Table() string {
	return r.tableName
}

// modelDB returns a GORM session scoped to the entity model and optional table.
// See sessionDB for why the trailing Session call is needed.
func (r Repository[D, E]) modelDB(ctx context.Context) *gorm.DB {
	db := r.db.Session(ctx).Model(new(E))
	if r.tableName != "" {
		db = db.Table(r.tableName).Session(&gorm.Session{})
	}
	return db
}

// sessionDB returns a GORM session scoped to the optional table (no Model).
// The trailing Session call resets the GORM clone counter so that callers
// get a fresh chainable session (without it, .Table() consumes the clone
// and subsequent chain methods mutate the session in place).
func (r Repository[D, E]) sessionDB(ctx context.Context) *gorm.DB {
	db := r.db.Session(ctx)
	if r.tableName != "" {
		db = db.Table(r.tableName).Session(&gorm.Session{})
	}
	return db
}

// Find retrieves entities matching the given options.
func (r Repository[D, E]) Find(ctx context.Context, options ...repository.Option) ([]D, error) {
	var entities []E
	db := ApplyOptions(r.modelDB(ctx), options...)
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
	db := ApplyOptions(r.sessionDB(ctx), options...)
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
	db := ApplyOptions(r.modelDB(ctx), options...)
	result := db.Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check %s exists: %w", r.label, result.Error)
	}
	return count > 0, nil
}

// DeleteBy removes entities matching the given options.
func (r Repository[D, E]) DeleteBy(ctx context.Context, options ...repository.Option) error {
	db := ApplyOptions(r.sessionDB(ctx), options...)
	result := db.Delete(new(E))
	if result.Error != nil {
		return fmt.Errorf("delete %s: %w", r.label, result.Error)
	}
	return nil
}

// DB returns a GORM session scoped to the optional dynamic table.
func (r Repository[D, E]) DB(ctx context.Context) *gorm.DB {
	return r.sessionDB(ctx)
}

// Count returns the number of entities matching the given options.
func (r Repository[D, E]) Count(ctx context.Context, options ...repository.Option) (int64, error) {
	var count int64
	db := ApplyConditions(r.modelDB(ctx), options...)
	if result := db.Count(&count); result.Error != nil {
		return 0, fmt.Errorf("count %s: %w", r.label, result.Error)
	}
	return count, nil
}

// Mapper returns the entity mapper for external use.
func (r Repository[D, E]) Mapper() EntityMapper[D, E] {
	return r.mapper
}
