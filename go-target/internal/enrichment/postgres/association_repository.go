package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/enrichment"
)

// AssociationRepository provides PostgreSQL storage for enrichment associations.
type AssociationRepository struct {
	db     database.Database
	mapper AssociationMapper
}

// NewAssociationRepository creates a new AssociationRepository.
func NewAssociationRepository(db database.Database) *AssociationRepository {
	return &AssociationRepository{
		db:     db,
		mapper: AssociationMapper{},
	}
}

// Get retrieves an association by ID.
func (r *AssociationRepository) Get(ctx context.Context, id int64) (enrichment.Association, error) {
	var entity AssociationEntity
	result := r.db.Session(ctx).Where("id = ?", id).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return enrichment.Association{}, fmt.Errorf("%w: association id %d", database.ErrNotFound, id)
		}
		return enrichment.Association{}, fmt.Errorf("get association: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Save creates or updates an association.
func (r *AssociationRepository) Save(ctx context.Context, assoc enrichment.Association) (enrichment.Association, error) {
	entity := r.mapper.ToDatabase(assoc)
	now := time.Now()

	if entity.ID == 0 {
		entity.CreatedAt = now
		entity.UpdatedAt = now
		result := r.db.Session(ctx).Create(&entity)
		if result.Error != nil {
			return enrichment.Association{}, fmt.Errorf("create association: %w", result.Error)
		}
	} else {
		entity.UpdatedAt = now
		result := r.db.Session(ctx).Save(&entity)
		if result.Error != nil {
			return enrichment.Association{}, fmt.Errorf("update association: %w", result.Error)
		}
	}

	return r.mapper.ToDomain(entity), nil
}

// Delete removes an association.
func (r *AssociationRepository) Delete(ctx context.Context, assoc enrichment.Association) error {
	entity := r.mapper.ToDatabase(assoc)
	result := r.db.Session(ctx).Delete(&entity)
	if result.Error != nil {
		return fmt.Errorf("delete association: %w", result.Error)
	}
	return nil
}

// FindByEnrichmentID returns all associations for a specific enrichment.
func (r *AssociationRepository) FindByEnrichmentID(ctx context.Context, enrichmentID int64) ([]enrichment.Association, error) {
	var entities []AssociationEntity
	result := r.db.Session(ctx).Where("enrichment_id = ?", enrichmentID).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find by enrichment id: %w", result.Error)
	}

	associations := make([]enrichment.Association, len(entities))
	for i, entity := range entities {
		associations[i] = r.mapper.ToDomain(entity)
	}
	return associations, nil
}

// FindByEntityID returns all associations for a specific entity.
func (r *AssociationRepository) FindByEntityID(ctx context.Context, entityID string) ([]enrichment.Association, error) {
	var entities []AssociationEntity
	result := r.db.Session(ctx).Where("entity_id = ?", entityID).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find by entity id: %w", result.Error)
	}

	associations := make([]enrichment.Association, len(entities))
	for i, entity := range entities {
		associations[i] = r.mapper.ToDomain(entity)
	}
	return associations, nil
}

// FindByEntityTypeAndID returns all associations for an entity of a specific type.
func (r *AssociationRepository) FindByEntityTypeAndID(ctx context.Context, entityType enrichment.EntityTypeKey, entityID string) ([]enrichment.Association, error) {
	var entities []AssociationEntity
	result := r.db.Session(ctx).Where("entity_type = ? AND entity_id = ?", string(entityType), entityID).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find by entity type and id: %w", result.Error)
	}

	associations := make([]enrichment.Association, len(entities))
	for i, entity := range entities {
		associations[i] = r.mapper.ToDomain(entity)
	}
	return associations, nil
}

// DeleteByEnrichmentID removes all associations for a specific enrichment.
func (r *AssociationRepository) DeleteByEnrichmentID(ctx context.Context, enrichmentID int64) error {
	result := r.db.Session(ctx).Where("enrichment_id = ?", enrichmentID).Delete(&AssociationEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete by enrichment id: %w", result.Error)
	}
	return nil
}

// DeleteByEntityID removes all associations for a specific entity.
func (r *AssociationRepository) DeleteByEntityID(ctx context.Context, entityID string) error {
	result := r.db.Session(ctx).Where("entity_id = ?", entityID).Delete(&AssociationEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete by entity id: %w", result.Error)
	}
	return nil
}

// Ensure AssociationRepository implements the interface.
var _ enrichment.AssociationRepository = (*AssociationRepository)(nil)
