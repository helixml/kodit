package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/database"
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

// FindByEntityKey returns all enrichments for a specific entity type (requires JOIN).
func (s EnrichmentStore) FindByEntityKey(ctx context.Context, key enrichment.EntityTypeKey) ([]enrichment.Enrichment, error) {
	var models []EnrichmentModel
	result := s.DB(ctx).
		Joins("JOIN enrichment_associations ON enrichment_associations.enrichment_id = enrichments_v2.id").
		Where("enrichment_associations.entity_type = ?", string(key)).
		Distinct().
		Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find by entity key: %w", result.Error)
	}

	enrichments := make([]enrichment.Enrichment, len(models))
	for i, model := range models {
		enrichments[i] = s.Mapper().ToDomain(model)
	}
	return enrichments, nil
}

// FindByCommitSHA returns enrichments associated with a commit via JOIN.
func (s EnrichmentStore) FindByCommitSHA(ctx context.Context, commitSHA string, options ...repository.Option) ([]enrichment.Enrichment, error) {
	var models []EnrichmentModel
	db := s.DB(ctx).
		Joins("JOIN enrichment_associations ON enrichment_associations.enrichment_id = enrichments_v2.id").
		Where("enrichment_associations.entity_type = ?", string(enrichment.EntityTypeCommit)).
		Where("enrichment_associations.entity_id = ?", commitSHA).
		Distinct()
	db = database.ApplyOptions(db, options...)
	result := db.Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find by commit SHA: %w", result.Error)
	}

	enrichments := make([]enrichment.Enrichment, len(models))
	for i, model := range models {
		enrichments[i] = s.Mapper().ToDomain(model)
	}
	return enrichments, nil
}

// CountByCommitSHA returns the count of enrichments for a commit.
func (s EnrichmentStore) CountByCommitSHA(ctx context.Context, commitSHA string, options ...repository.Option) (int64, error) {
	var count int64
	db := s.DB(ctx).Model(&EnrichmentModel{}).
		Joins("JOIN enrichment_associations ON enrichment_associations.enrichment_id = enrichments_v2.id").
		Where("enrichment_associations.entity_type = ?", string(enrichment.EntityTypeCommit)).
		Where("enrichment_associations.entity_id = ?", commitSHA)
	db = database.ApplyConditions(db, options...)
	result := db.Distinct("enrichments_v2.id").Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count by commit SHA: %w", result.Error)
	}
	return count, nil
}

// FindByCommitSHAs returns enrichments across multiple commits via JOIN.
func (s EnrichmentStore) FindByCommitSHAs(ctx context.Context, commitSHAs []string, options ...repository.Option) ([]enrichment.Enrichment, error) {
	if len(commitSHAs) == 0 {
		return []enrichment.Enrichment{}, nil
	}

	var models []EnrichmentModel
	db := s.DB(ctx).
		Joins("JOIN enrichment_associations ON enrichment_associations.enrichment_id = enrichments_v2.id").
		Where("enrichment_associations.entity_type = ?", string(enrichment.EntityTypeCommit)).
		Where("enrichment_associations.entity_id IN ?", commitSHAs).
		Distinct()
	db = database.ApplyOptions(db, options...)
	result := db.Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find by commit SHAs: %w", result.Error)
	}

	enrichments := make([]enrichment.Enrichment, len(models))
	for i, model := range models {
		enrichments[i] = s.Mapper().ToDomain(model)
	}
	return enrichments, nil
}

// CountByCommitSHAs returns the count of enrichments across multiple commits.
func (s EnrichmentStore) CountByCommitSHAs(ctx context.Context, commitSHAs []string, options ...repository.Option) (int64, error) {
	if len(commitSHAs) == 0 {
		return 0, nil
	}

	var count int64
	db := s.DB(ctx).Model(&EnrichmentModel{}).
		Joins("JOIN enrichment_associations ON enrichment_associations.enrichment_id = enrichments_v2.id").
		Where("enrichment_associations.entity_type = ?", string(enrichment.EntityTypeCommit)).
		Where("enrichment_associations.entity_id IN ?", commitSHAs)
	db = database.ApplyConditions(db, options...)
	result := db.Distinct("enrichments_v2.id").Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count by commit SHAs: %w", result.Error)
	}
	return count, nil
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
