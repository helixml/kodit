package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/persistence/models"
	"github.com/helixml/kodit/internal/database"

	"gorm.io/gorm"
)

// PipelineMapper maps between domain Pipeline and persistence models.
type PipelineMapper struct{}

// ToDomain converts a models.Pipeline to a domain Pipeline.
func (m PipelineMapper) ToDomain(e models.Pipeline) repository.Pipeline {
	steps := make([]repository.Step, len(e.Steps))
	for i, s := range e.Steps {
		deps := make([]int64, len(s.Dependencies))
		for j, d := range s.Dependencies {
			deps[j] = int64(d.DependsOnID)
		}
		steps[i] = repository.ReconstructStep(
			int64(s.ID),
			s.Kind,
			deps,
			s.CreatedAt,
			s.UpdatedAt,
		)
	}
	return repository.ReconstructPipeline(
		int64(e.ID),
		e.RepoID,
		steps,
		e.CreatedAt,
		e.UpdatedAt,
	)
}

// ToModel converts a domain Pipeline to a models.Pipeline.
func (m PipelineMapper) ToModel(p repository.Pipeline) models.Pipeline {
	now := time.Now()
	steps := make([]models.Step, len(p.Steps()))
	for i, s := range p.Steps() {
		deps := make([]models.StepDependency, len(s.Dependencies()))
		for j, depID := range s.Dependencies() {
			deps[j] = models.StepDependency{
				DependsOnID: uint(depID),
			}
		}
		step := models.Step{
			Kind:         s.Kind(),
			Dependencies: deps,
		}
		if s.ID() != 0 {
			step.ID = uint(s.ID())
			step.CreatedAt = s.CreatedAt()
			step.UpdatedAt = now
		}
		steps[i] = step
	}
	pipeline := models.Pipeline{
		RepoID: p.RepoID(),
		Steps:  steps,
	}
	if p.ID() != 0 {
		pipeline.ID = uint(p.ID())
		pipeline.CreatedAt = p.CreatedAt()
		pipeline.UpdatedAt = now
	}
	return pipeline
}

// PipelineStore implements repository.PipelineStore using GORM.
type PipelineStore struct {
	database.Repository[repository.Pipeline, models.Pipeline]
}

// NewPipelineStore creates a new PipelineStore.
func NewPipelineStore(db database.Database) PipelineStore {
	return PipelineStore{
		Repository: database.NewRepository(db, PipelineMapper{}, "pipeline"),
	}
}

// Save creates or updates a pipeline with its steps and dependencies.
func (s PipelineStore) Save(ctx context.Context, pipeline repository.Pipeline) (repository.Pipeline, error) {
	model := s.Mapper().ToModel(pipeline)

	err := s.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if pipeline.ID() != 0 {
			// Delete existing steps; CASCADE removes dependencies.
			if err := tx.Where("pipeline_id = ?", pipeline.ID()).Delete(&models.Step{}).Error; err != nil {
				return fmt.Errorf("delete old steps: %w", err)
			}
			// Clear step IDs so GORM inserts fresh rows.
			for i := range model.Steps {
				model.Steps[i].ID = 0
				for j := range model.Steps[i].Dependencies {
					model.Steps[i].Dependencies[j].ID = 0
				}
			}
			return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&model).Error
		}
		return tx.Create(&model).Error
	})
	if err != nil {
		return repository.Pipeline{}, fmt.Errorf("save pipeline: %w", err)
	}

	return s.Mapper().ToDomain(model), nil
}
