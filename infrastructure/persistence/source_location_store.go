package persistence

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/sourcelocation"
	"github.com/helixml/kodit/internal/database"
)

// SourceLocationStore implements sourcelocation.Store using GORM.
type SourceLocationStore struct {
	database.Repository[sourcelocation.SourceLocation, SourceLocationModel]
}

// NewSourceLocationStore creates a new SourceLocationStore.
func NewSourceLocationStore(db database.Database) SourceLocationStore {
	return SourceLocationStore{
		Repository: database.NewRepository[sourcelocation.SourceLocation, SourceLocationModel](db, SourceLocationMapper{}, "source_location"),
	}
}

// Save creates or updates a source location.
func (s SourceLocationStore) Save(ctx context.Context, sl sourcelocation.SourceLocation) (sourcelocation.SourceLocation, error) {
	model := s.Mapper().ToModel(sl)

	if model.ID == 0 {
		result := s.DB(ctx).Create(&model)
		if result.Error != nil {
			return sourcelocation.SourceLocation{}, fmt.Errorf("create source location: %w", result.Error)
		}
	} else {
		result := s.DB(ctx).Save(&model)
		if result.Error != nil {
			return sourcelocation.SourceLocation{}, fmt.Errorf("update source location: %w", result.Error)
		}
	}

	return s.Mapper().ToDomain(model), nil
}

// Delete removes a source location.
func (s SourceLocationStore) Delete(ctx context.Context, sl sourcelocation.SourceLocation) error {
	model := s.Mapper().ToModel(sl)
	result := s.DB(ctx).Delete(&model)
	if result.Error != nil {
		return fmt.Errorf("delete source location: %w", result.Error)
	}
	return nil
}
