package persistence

import (
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
