package enrichment

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// EnrichmentStore defines operations for persisting and retrieving enrichments.
type EnrichmentStore interface {
	repository.Store[Enrichment]
	DeleteBy(ctx context.Context, options ...repository.Option) error

	// FindByEntityKey returns all enrichments for a specific entity type (requires JOIN).
	FindByEntityKey(ctx context.Context, key EntityTypeKey) ([]Enrichment, error)
}

// AssociationStore defines operations for persisting and retrieving enrichment associations.
type AssociationStore interface {
	repository.Store[Association]
	DeleteBy(ctx context.Context, options ...repository.Option) error
}
