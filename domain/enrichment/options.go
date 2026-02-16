package enrichment

import "github.com/helixml/kodit/domain/repository"

// WithType filters by the "type" column.
func WithType(typ Type) repository.Option {
	return repository.WithCondition("type", string(typ))
}

// WithSubtype filters by the "subtype" column.
func WithSubtype(subtype Subtype) repository.Option {
	return repository.WithCondition("subtype", string(subtype))
}

// WithEnrichmentID filters by the "enrichment_id" column.
func WithEnrichmentID(id int64) repository.Option {
	return repository.WithCondition("enrichment_id", id)
}

// WithEntityID filters by the "entity_id" column.
func WithEntityID(entityID string) repository.Option {
	return repository.WithCondition("entity_id", entityID)
}

// WithEntityType filters by the "entity_type" column.
func WithEntityType(entityType EntityTypeKey) repository.Option {
	return repository.WithCondition("entity_type", string(entityType))
}

// WithEntityIDIn filters by multiple entity IDs.
func WithEntityIDIn(entityIDs []string) repository.Option {
	return repository.WithConditionIn("entity_id", entityIDs)
}

// WithEnrichmentIDIn filters by multiple enrichment IDs.
func WithEnrichmentIDIn(ids []int64) repository.Option {
	return repository.WithConditionIn("enrichment_id", ids)
}
