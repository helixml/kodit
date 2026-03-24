package persistence

import (
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/persistence/models"
	"github.com/helixml/kodit/internal/database"
)

// PipelineMapper maps between domain Pipeline and persistence models.
type PipelineMapper struct{}

// ToDomain converts a models.Pipeline to a domain Pipeline.
func (m PipelineMapper) ToDomain(e models.Pipeline) repository.Pipeline {
	return repository.ReconstructPipeline(
		int64(e.ID),
		e.Name,
		e.CreatedAt,
		e.UpdatedAt,
	)
}

// ToModel converts a domain Pipeline to a models.Pipeline.
func (m PipelineMapper) ToModel(p repository.Pipeline) models.Pipeline {
	pipeline := models.Pipeline{
		Name: p.Name(),
	}
	if p.ID() != 0 {
		pipeline.ID = uint(p.ID())
		pipeline.CreatedAt = p.CreatedAt()
		pipeline.UpdatedAt = time.Now()
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

// StepMapper maps between domain Step and persistence models.
type StepMapper struct{}

// ToDomain converts a models.Step to a domain Step.
func (m StepMapper) ToDomain(e models.Step) repository.Step {
	return repository.ReconstructStep(
		int64(e.ID),
		int64(e.PipelineID),
		e.Name,
		e.Kind,
		e.CreatedAt,
		e.UpdatedAt,
	)
}

// ToModel converts a domain Step to a models.Step.
func (m StepMapper) ToModel(s repository.Step) models.Step {
	step := models.Step{
		PipelineID: uint(s.PipelineID()),
		Name:       s.Name(),
		Kind:       s.Kind(),
	}
	if s.ID() != 0 {
		step.ID = uint(s.ID())
		step.CreatedAt = s.CreatedAt()
		step.UpdatedAt = time.Now()
	}
	return step
}

// StepStore implements repository.StepStore using GORM.
type StepStore struct {
	database.Repository[repository.Step, models.Step]
}

// NewStepStore creates a new StepStore.
func NewStepStore(db database.Database) StepStore {
	return StepStore{
		Repository: database.NewRepository(db, StepMapper{}, "step"),
	}
}

// StepDependencyMapper maps between domain StepDependency and persistence models.
type StepDependencyMapper struct{}

// ToDomain converts a models.StepDependency to a domain StepDependency.
func (m StepDependencyMapper) ToDomain(e models.StepDependency) repository.StepDependency {
	return repository.ReconstructStepDependency(
		int64(e.ID),
		int64(e.StepID),
		int64(e.DependsOnID),
		e.CreatedAt,
		e.UpdatedAt,
	)
}

// ToModel converts a domain StepDependency to a models.StepDependency.
func (m StepDependencyMapper) ToModel(d repository.StepDependency) models.StepDependency {
	dep := models.StepDependency{
		StepID:      uint(d.StepID()),
		DependsOnID: uint(d.DependsOnID()),
	}
	if d.ID() != 0 {
		dep.ID = uint(d.ID())
		dep.CreatedAt = d.CreatedAt()
		dep.UpdatedAt = time.Now()
	}
	return dep
}

// StepDependencyStore implements repository.StepDependencyStore using GORM.
type StepDependencyStore struct {
	database.Repository[repository.StepDependency, models.StepDependency]
}

// NewStepDependencyStore creates a new StepDependencyStore.
func NewStepDependencyStore(db database.Database) StepDependencyStore {
	return StepDependencyStore{
		Repository: database.NewRepository(db, StepDependencyMapper{}, "step dependency"),
	}
}
