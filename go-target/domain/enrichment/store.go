package enrichment

import (
	"context"
)

// EnrichmentStore defines operations for persisting and retrieving enrichments.
type EnrichmentStore interface {
	// Get retrieves an enrichment by ID.
	Get(ctx context.Context, id int64) (Enrichment, error)

	// Save creates or updates an enrichment.
	Save(ctx context.Context, enrichment Enrichment) (Enrichment, error)

	// Delete removes an enrichment.
	Delete(ctx context.Context, enrichment Enrichment) error

	// DeleteByIDs removes enrichments by their IDs.
	DeleteByIDs(ctx context.Context, ids []int64) error

	// FindByType returns all enrichments of a specific type.
	FindByType(ctx context.Context, typ Type) ([]Enrichment, error)

	// FindByTypeAndSubtype returns all enrichments of a specific type and subtype.
	FindByTypeAndSubtype(ctx context.Context, typ Type, subtype Subtype) ([]Enrichment, error)

	// FindByEntityKey returns all enrichments for a specific entity type.
	FindByEntityKey(ctx context.Context, key EntityTypeKey) ([]Enrichment, error)
}

// AssociationStore defines operations for persisting and retrieving enrichment associations.
type AssociationStore interface {
	// Get retrieves an association by ID.
	Get(ctx context.Context, id int64) (Association, error)

	// Save creates or updates an association.
	Save(ctx context.Context, assoc Association) (Association, error)

	// Delete removes an association.
	Delete(ctx context.Context, assoc Association) error

	// FindByEnrichmentID returns all associations for a specific enrichment.
	FindByEnrichmentID(ctx context.Context, enrichmentID int64) ([]Association, error)

	// FindByEntityID returns all associations for a specific entity.
	FindByEntityID(ctx context.Context, entityID string) ([]Association, error)

	// FindByEntityTypeAndID returns all associations for an entity of a specific type.
	FindByEntityTypeAndID(ctx context.Context, entityType EntityTypeKey, entityID string) ([]Association, error)

	// DeleteByEnrichmentID removes all associations for a specific enrichment.
	DeleteByEnrichmentID(ctx context.Context, enrichmentID int64) error

	// DeleteByEntityID removes all associations for a specific entity.
	DeleteByEntityID(ctx context.Context, entityID string) error
}
