package postgres

import (
	"github.com/helixml/kodit/internal/enrichment"
)

// EnrichmentMapper maps between Enrichment domain objects and database entities.
type EnrichmentMapper struct{}

// ToDomain converts a database entity to a domain object.
func (m EnrichmentMapper) ToDomain(entity EnrichmentEntity) enrichment.Enrichment {
	return enrichment.ReconstructEnrichment(
		entity.ID,
		enrichment.Type(entity.Type),
		enrichment.Subtype(entity.Subtype),
		enrichment.EntityTypeCommit, // Default - actual entity type comes from associations
		entity.Content,
		"", // Language not stored in DB
		entity.CreatedAt,
		entity.UpdatedAt,
	)
}

// ToDatabase converts a domain object to a database entity.
func (m EnrichmentMapper) ToDatabase(domain enrichment.Enrichment) EnrichmentEntity {
	return EnrichmentEntity{
		ID:        domain.ID(),
		Type:      string(domain.Type()),
		Subtype:   string(domain.Subtype()),
		Content:   domain.Content(),
		CreatedAt: domain.CreatedAt(),
		UpdatedAt: domain.UpdatedAt(),
	}
}

// AssociationMapper maps between Association domain objects and database entities.
type AssociationMapper struct{}

// ToDomain converts a database entity to a domain object.
func (m AssociationMapper) ToDomain(entity AssociationEntity) enrichment.Association {
	return enrichment.ReconstructAssociation(
		entity.ID,
		entity.EnrichmentID,
		entity.EntityID,
		enrichment.EntityTypeKey(entity.EntityType),
	)
}

// ToDatabase converts a domain object to a database entity.
func (m AssociationMapper) ToDatabase(domain enrichment.Association) AssociationEntity {
	return AssociationEntity{
		ID:           domain.ID(),
		EnrichmentID: domain.EnrichmentID(),
		EntityType:   string(domain.EntityType()),
		EntityID:     domain.EntityID(),
	}
}
