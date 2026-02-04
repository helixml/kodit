package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/enrichment"
	"gorm.io/gorm"
)

// EnrichmentStore implements enrichment.EnrichmentStore using GORM.
type EnrichmentStore struct {
	db     Database
	mapper EnrichmentMapper
}

// NewEnrichmentStore creates a new EnrichmentStore.
func NewEnrichmentStore(db Database) EnrichmentStore {
	return EnrichmentStore{
		db:     db,
		mapper: EnrichmentMapper{},
	}
}

// Get retrieves an enrichment by ID.
func (s EnrichmentStore) Get(ctx context.Context, id int64) (enrichment.Enrichment, error) {
	var model EnrichmentModel
	result := s.db.Session(ctx).Where("id = ?", id).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return enrichment.Enrichment{}, fmt.Errorf("%w: enrichment id %d", ErrNotFound, id)
		}
		return enrichment.Enrichment{}, fmt.Errorf("get enrichment: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// Save creates or updates an enrichment.
func (s EnrichmentStore) Save(ctx context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error) {
	model := s.mapper.ToModel(e)
	now := time.Now()

	if model.ID == 0 {
		model.CreatedAt = now
		model.UpdatedAt = now
		result := s.db.Session(ctx).Create(&model)
		if result.Error != nil {
			return enrichment.Enrichment{}, fmt.Errorf("create enrichment: %w", result.Error)
		}
	} else {
		model.UpdatedAt = now
		result := s.db.Session(ctx).Save(&model)
		if result.Error != nil {
			return enrichment.Enrichment{}, fmt.Errorf("update enrichment: %w", result.Error)
		}
	}

	return s.mapper.ToDomain(model), nil
}

// Delete removes an enrichment.
func (s EnrichmentStore) Delete(ctx context.Context, e enrichment.Enrichment) error {
	model := s.mapper.ToModel(e)
	result := s.db.Session(ctx).Delete(&model)
	if result.Error != nil {
		return fmt.Errorf("delete enrichment: %w", result.Error)
	}
	return nil
}

// FindByType returns all enrichments of a specific type.
func (s EnrichmentStore) FindByType(ctx context.Context, typ enrichment.Type) ([]enrichment.Enrichment, error) {
	var models []EnrichmentModel
	result := s.db.Session(ctx).Where("type = ?", string(typ)).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find by type: %w", result.Error)
	}

	enrichments := make([]enrichment.Enrichment, len(models))
	for i, model := range models {
		enrichments[i] = s.mapper.ToDomain(model)
	}
	return enrichments, nil
}

// FindByTypeAndSubtype returns all enrichments of a specific type and subtype.
func (s EnrichmentStore) FindByTypeAndSubtype(ctx context.Context, typ enrichment.Type, subtype enrichment.Subtype) ([]enrichment.Enrichment, error) {
	var models []EnrichmentModel
	result := s.db.Session(ctx).Where("type = ? AND subtype = ?", string(typ), string(subtype)).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find by type and subtype: %w", result.Error)
	}

	enrichments := make([]enrichment.Enrichment, len(models))
	for i, model := range models {
		enrichments[i] = s.mapper.ToDomain(model)
	}
	return enrichments, nil
}

// FindByEntityKey returns all enrichments for a specific entity type.
func (s EnrichmentStore) FindByEntityKey(ctx context.Context, key enrichment.EntityTypeKey) ([]enrichment.Enrichment, error) {
	var models []EnrichmentModel
	result := s.db.Session(ctx).
		Joins("JOIN enrichment_associations ON enrichment_associations.enrichment_id = enrichments_v2.id").
		Where("enrichment_associations.entity_type = ?", string(key)).
		Distinct().
		Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find by entity key: %w", result.Error)
	}

	enrichments := make([]enrichment.Enrichment, len(models))
	for i, model := range models {
		enrichments[i] = s.mapper.ToDomain(model)
	}
	return enrichments, nil
}

// AssociationStore implements enrichment.AssociationStore using GORM.
type AssociationStore struct {
	db     Database
	mapper AssociationMapper
}

// NewAssociationStore creates a new AssociationStore.
func NewAssociationStore(db Database) AssociationStore {
	return AssociationStore{
		db:     db,
		mapper: AssociationMapper{},
	}
}

// Get retrieves an association by ID.
func (s AssociationStore) Get(ctx context.Context, id int64) (enrichment.Association, error) {
	var model EnrichmentAssociationModel
	result := s.db.Session(ctx).Where("id = ?", id).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return enrichment.Association{}, fmt.Errorf("%w: association id %d", ErrNotFound, id)
		}
		return enrichment.Association{}, fmt.Errorf("get association: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// Save creates or updates an association.
func (s AssociationStore) Save(ctx context.Context, assoc enrichment.Association) (enrichment.Association, error) {
	model := s.mapper.ToModel(assoc)

	if model.ID == 0 {
		result := s.db.Session(ctx).Create(&model)
		if result.Error != nil {
			return enrichment.Association{}, fmt.Errorf("create association: %w", result.Error)
		}
	} else {
		result := s.db.Session(ctx).Save(&model)
		if result.Error != nil {
			return enrichment.Association{}, fmt.Errorf("update association: %w", result.Error)
		}
	}

	return s.mapper.ToDomain(model), nil
}

// Delete removes an association.
func (s AssociationStore) Delete(ctx context.Context, assoc enrichment.Association) error {
	model := s.mapper.ToModel(assoc)
	result := s.db.Session(ctx).Delete(&model)
	if result.Error != nil {
		return fmt.Errorf("delete association: %w", result.Error)
	}
	return nil
}

// FindByEnrichmentID returns all associations for a specific enrichment.
func (s AssociationStore) FindByEnrichmentID(ctx context.Context, enrichmentID int64) ([]enrichment.Association, error) {
	var models []EnrichmentAssociationModel
	result := s.db.Session(ctx).Where("enrichment_id = ?", enrichmentID).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find by enrichment id: %w", result.Error)
	}

	associations := make([]enrichment.Association, len(models))
	for i, model := range models {
		associations[i] = s.mapper.ToDomain(model)
	}
	return associations, nil
}

// FindByEntityID returns all associations for a specific entity.
func (s AssociationStore) FindByEntityID(ctx context.Context, entityID string) ([]enrichment.Association, error) {
	var models []EnrichmentAssociationModel
	result := s.db.Session(ctx).Where("entity_id = ?", entityID).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find by entity id: %w", result.Error)
	}

	associations := make([]enrichment.Association, len(models))
	for i, model := range models {
		associations[i] = s.mapper.ToDomain(model)
	}
	return associations, nil
}

// FindByEntityTypeAndID returns all associations for an entity of a specific type.
func (s AssociationStore) FindByEntityTypeAndID(ctx context.Context, entityType enrichment.EntityTypeKey, entityID string) ([]enrichment.Association, error) {
	var models []EnrichmentAssociationModel
	result := s.db.Session(ctx).Where("entity_type = ? AND entity_id = ?", string(entityType), entityID).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find by entity type and id: %w", result.Error)
	}

	associations := make([]enrichment.Association, len(models))
	for i, model := range models {
		associations[i] = s.mapper.ToDomain(model)
	}
	return associations, nil
}

// DeleteByEnrichmentID removes all associations for a specific enrichment.
func (s AssociationStore) DeleteByEnrichmentID(ctx context.Context, enrichmentID int64) error {
	result := s.db.Session(ctx).Where("enrichment_id = ?", enrichmentID).Delete(&EnrichmentAssociationModel{})
	if result.Error != nil {
		return fmt.Errorf("delete by enrichment id: %w", result.Error)
	}
	return nil
}

// DeleteByEntityID removes all associations for a specific entity.
func (s AssociationStore) DeleteByEntityID(ctx context.Context, entityID string) error {
	result := s.db.Session(ctx).Where("entity_id = ?", entityID).Delete(&EnrichmentAssociationModel{})
	if result.Error != nil {
		return fmt.Errorf("delete by entity id: %w", result.Error)
	}
	return nil
}
