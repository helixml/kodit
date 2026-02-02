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

// EnrichmentRepository provides PostgreSQL storage for enrichments.
type EnrichmentRepository struct {
	db     database.Database
	mapper EnrichmentMapper
}

// NewEnrichmentRepository creates a new EnrichmentRepository.
func NewEnrichmentRepository(db database.Database) *EnrichmentRepository {
	return &EnrichmentRepository{
		db:     db,
		mapper: EnrichmentMapper{},
	}
}

// Get retrieves an enrichment by ID.
func (r *EnrichmentRepository) Get(ctx context.Context, id int64) (enrichment.Enrichment, error) {
	var entity EnrichmentEntity
	result := r.db.Session(ctx).Where("id = ?", id).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return enrichment.Enrichment{}, fmt.Errorf("%w: enrichment id %d", database.ErrNotFound, id)
		}
		return enrichment.Enrichment{}, fmt.Errorf("get enrichment: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Save creates or updates an enrichment.
func (r *EnrichmentRepository) Save(ctx context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error) {
	entity := r.mapper.ToDatabase(e)
	now := time.Now()

	if entity.ID == 0 {
		entity.CreatedAt = now
		entity.UpdatedAt = now
		result := r.db.Session(ctx).Create(&entity)
		if result.Error != nil {
			return enrichment.Enrichment{}, fmt.Errorf("create enrichment: %w", result.Error)
		}
	} else {
		entity.UpdatedAt = now
		result := r.db.Session(ctx).Save(&entity)
		if result.Error != nil {
			return enrichment.Enrichment{}, fmt.Errorf("update enrichment: %w", result.Error)
		}
	}

	return r.mapper.ToDomain(entity), nil
}

// Delete removes an enrichment.
func (r *EnrichmentRepository) Delete(ctx context.Context, e enrichment.Enrichment) error {
	entity := r.mapper.ToDatabase(e)
	result := r.db.Session(ctx).Delete(&entity)
	if result.Error != nil {
		return fmt.Errorf("delete enrichment: %w", result.Error)
	}
	return nil
}

// FindByType returns all enrichments of a specific type.
func (r *EnrichmentRepository) FindByType(ctx context.Context, typ enrichment.Type) ([]enrichment.Enrichment, error) {
	var entities []EnrichmentEntity
	result := r.db.Session(ctx).Where("type = ?", string(typ)).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find by type: %w", result.Error)
	}

	enrichments := make([]enrichment.Enrichment, len(entities))
	for i, entity := range entities {
		enrichments[i] = r.mapper.ToDomain(entity)
	}
	return enrichments, nil
}

// FindByTypeAndSubtype returns all enrichments of a specific type and subtype.
func (r *EnrichmentRepository) FindByTypeAndSubtype(ctx context.Context, typ enrichment.Type, subtype enrichment.Subtype) ([]enrichment.Enrichment, error) {
	var entities []EnrichmentEntity
	result := r.db.Session(ctx).Where("type = ? AND subtype = ?", string(typ), string(subtype)).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find by type and subtype: %w", result.Error)
	}

	enrichments := make([]enrichment.Enrichment, len(entities))
	for i, entity := range entities {
		enrichments[i] = r.mapper.ToDomain(entity)
	}
	return enrichments, nil
}

// FindByEntityKey returns all enrichments for a specific entity type.
// This requires joining with associations to find enrichments linked to that entity type.
func (r *EnrichmentRepository) FindByEntityKey(ctx context.Context, key enrichment.EntityTypeKey) ([]enrichment.Enrichment, error) {
	var entities []EnrichmentEntity
	result := r.db.Session(ctx).
		Joins("JOIN enrichment_associations ON enrichment_associations.enrichment_id = enrichments_v2.id").
		Where("enrichment_associations.entity_type = ?", string(key)).
		Distinct().
		Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find by entity key: %w", result.Error)
	}

	enrichments := make([]enrichment.Enrichment, len(entities))
	for i, entity := range entities {
		enrichments[i] = r.mapper.ToDomain(entity)
	}
	return enrichments, nil
}

// Ensure EnrichmentRepository implements the interface.
var _ enrichment.EnrichmentRepository = (*EnrichmentRepository)(nil)
