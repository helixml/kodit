package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

// EnrichmentStore implements enrichment.EnrichmentStore using GORM.
type EnrichmentStore struct {
	database.Repository[enrichment.Enrichment, EnrichmentModel]
}

// NewEnrichmentStore creates a new EnrichmentStore.
func NewEnrichmentStore(db database.Database) EnrichmentStore {
	return EnrichmentStore{
		Repository: database.NewRepository[enrichment.Enrichment, EnrichmentModel](db, EnrichmentMapper{}, "enrichment"),
	}
}

// Save creates or updates an enrichment.
func (s EnrichmentStore) Save(ctx context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error) {
	model := s.Mapper().ToModel(e)
	now := time.Now()

	if model.ID == 0 {
		model.CreatedAt = now
		model.UpdatedAt = now
		result := s.DB(ctx).Create(&model)
		if result.Error != nil {
			return enrichment.Enrichment{}, fmt.Errorf("create enrichment: %w", result.Error)
		}
	} else {
		model.UpdatedAt = now
		result := s.DB(ctx).Save(&model)
		if result.Error != nil {
			return enrichment.Enrichment{}, fmt.Errorf("update enrichment: %w", result.Error)
		}
	}

	return s.Mapper().ToDomain(model), nil
}

// Delete removes an enrichment.
func (s EnrichmentStore) Delete(ctx context.Context, e enrichment.Enrichment) error {
	model := s.Mapper().ToModel(e)
	result := s.DB(ctx).Delete(&model)
	if result.Error != nil {
		return fmt.Errorf("delete enrichment: %w", result.Error)
	}
	return nil
}

// Find retrieves enrichments matching the given options.
// Supports commit SHA filtering via enrichment.WithCommitSHA / WithCommitSHAs
// options, which join through enrichment_associations.
func (s EnrichmentStore) Find(ctx context.Context, options ...repository.Option) ([]enrichment.Enrichment, error) {
	q := repository.Build(options...)
	db := s.commitJoin(s.DB(ctx), q)
	db = database.ApplyOptions(db, options...)

	var models []EnrichmentModel
	if err := db.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("find enrichments: %w", err)
	}

	enrichments := make([]enrichment.Enrichment, len(models))
	for i, model := range models {
		enrichments[i] = s.Mapper().ToDomain(model)
	}
	return enrichments, nil
}

// Count returns the number of enrichments matching the given options.
func (s EnrichmentStore) Count(ctx context.Context, options ...repository.Option) (int64, error) {
	q := repository.Build(options...)
	db := s.commitJoin(s.DB(ctx).Model(&EnrichmentModel{}), q)
	db = database.ApplyConditions(db, options...)

	var count int64
	if needsCommitJoin(q) {
		result := db.Distinct("enrichments_v2.id").Count(&count)
		return count, result.Error
	}
	result := db.Count(&count)
	return count, result.Error
}

// commitJoin applies the enrichment_associations JOIN when commit SHA
// options are present in the query.
func (s EnrichmentStore) commitJoin(db *gorm.DB, q repository.Query) *gorm.DB {
	if sha, ok := enrichment.CommitSHAFrom(q); ok {
		return db.
			Joins("JOIN enrichment_associations ON enrichment_associations.enrichment_id = enrichments_v2.id").
			Where("enrichment_associations.entity_type = ?", string(enrichment.EntityTypeCommit)).
			Where("enrichment_associations.entity_id = ?", sha).
			Distinct()
	}
	if shas, ok := enrichment.CommitSHAsFrom(q); ok {
		return db.
			Joins("JOIN enrichment_associations ON enrichment_associations.enrichment_id = enrichments_v2.id").
			Where("enrichment_associations.entity_type = ?", string(enrichment.EntityTypeCommit)).
			Where("enrichment_associations.entity_id IN ?", shas).
			Distinct()
	}
	return db
}

// needsCommitJoin returns true when commit SHA filtering is active.
func needsCommitJoin(q repository.Query) bool {
	if _, ok := enrichment.CommitSHAFrom(q); ok {
		return true
	}
	_, ok := enrichment.CommitSHAsFrom(q)
	return ok
}

// AssociationStore implements enrichment.AssociationStore using GORM.
type AssociationStore struct {
	database.Repository[enrichment.Association, EnrichmentAssociationModel]
}

// NewAssociationStore creates a new AssociationStore.
func NewAssociationStore(db database.Database) AssociationStore {
	return AssociationStore{
		Repository: database.NewRepository[enrichment.Association, EnrichmentAssociationModel](db, AssociationMapper{}, "association"),
	}
}

// Save creates or updates an association.
func (s AssociationStore) Save(ctx context.Context, assoc enrichment.Association) (enrichment.Association, error) {
	model := s.Mapper().ToModel(assoc)

	if model.ID == 0 {
		result := s.DB(ctx).Create(&model)
		if result.Error != nil {
			return enrichment.Association{}, fmt.Errorf("create association: %w", result.Error)
		}
	} else {
		result := s.DB(ctx).Save(&model)
		if result.Error != nil {
			return enrichment.Association{}, fmt.Errorf("update association: %w", result.Error)
		}
	}

	return s.Mapper().ToDomain(model), nil
}

// Delete removes an association.
func (s AssociationStore) Delete(ctx context.Context, assoc enrichment.Association) error {
	model := s.Mapper().ToModel(assoc)
	result := s.DB(ctx).Delete(&model)
	if result.Error != nil {
		return fmt.Errorf("delete association: %w", result.Error)
	}
	return nil
}
