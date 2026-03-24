package repository

import "time"

// Pipeline represents a processing pipeline.
type Pipeline struct {
	id        int64
	name      string
	createdAt time.Time
	updatedAt time.Time
}

// NewPipeline creates a new Pipeline with the given name.
func NewPipeline(name string) Pipeline {
	now := time.Now()
	return Pipeline{
		name:      name,
		createdAt: now,
		updatedAt: now,
	}
}

// ReconstructPipeline rebuilds a Pipeline from persisted data.
func ReconstructPipeline(id int64, name string, createdAt, updatedAt time.Time) Pipeline {
	return Pipeline{
		id:        id,
		name:      name,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// ID returns the pipeline identifier.
func (p Pipeline) ID() int64 { return p.id }

// Name returns the human-readable pipeline name.
func (p Pipeline) Name() string { return p.name }

// CreatedAt returns the creation timestamp.
func (p Pipeline) CreatedAt() time.Time { return p.createdAt }

// UpdatedAt returns the last update timestamp.
func (p Pipeline) UpdatedAt() time.Time { return p.updatedAt }

// Step represents a single step in a pipeline.
type Step struct {
	id         int64
	pipelineID int64
	name       string
	kind       string
	createdAt  time.Time
	updatedAt  time.Time
}

// NewStep creates a new Step for the given pipeline.
func NewStep(pipelineID int64, name, kind string) Step {
	now := time.Now()
	return Step{
		pipelineID: pipelineID,
		name:       name,
		kind:       kind,
		createdAt:  now,
		updatedAt:  now,
	}
}

// ReconstructStep rebuilds a Step from persisted data.
func ReconstructStep(id, pipelineID int64, name, kind string, createdAt, updatedAt time.Time) Step {
	return Step{
		id:         id,
		pipelineID: pipelineID,
		name:       name,
		kind:       kind,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

// ID returns the step identifier.
func (s Step) ID() int64 { return s.id }

// PipelineID returns the associated pipeline identifier.
func (s Step) PipelineID() int64 { return s.pipelineID }

// Name returns the step name.
func (s Step) Name() string { return s.name }

// Kind returns the step kind.
func (s Step) Kind() string { return s.kind }

// CreatedAt returns the creation timestamp.
func (s Step) CreatedAt() time.Time { return s.createdAt }

// UpdatedAt returns the last update timestamp.
func (s Step) UpdatedAt() time.Time { return s.updatedAt }

// StepDependency links a step to another step it depends on.
type StepDependency struct {
	id          int64
	stepID      int64
	dependsOnID int64
	createdAt   time.Time
	updatedAt   time.Time
}

// NewStepDependency creates a new dependency between two steps.
func NewStepDependency(stepID, dependsOnID int64) StepDependency {
	now := time.Now()
	return StepDependency{
		stepID:      stepID,
		dependsOnID: dependsOnID,
		createdAt:   now,
		updatedAt:   now,
	}
}

// ReconstructStepDependency rebuilds a StepDependency from persisted data.
func ReconstructStepDependency(id, stepID, dependsOnID int64, createdAt, updatedAt time.Time) StepDependency {
	return StepDependency{
		id:          id,
		stepID:      stepID,
		dependsOnID: dependsOnID,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}

// ID returns the dependency identifier.
func (d StepDependency) ID() int64 { return d.id }

// StepID returns the step that has the dependency.
func (d StepDependency) StepID() int64 { return d.stepID }

// DependsOnID returns the step that is depended on.
func (d StepDependency) DependsOnID() int64 { return d.dependsOnID }

// CreatedAt returns the creation timestamp.
func (d StepDependency) CreatedAt() time.Time { return d.createdAt }

// UpdatedAt returns the last update timestamp.
func (d StepDependency) UpdatedAt() time.Time { return d.updatedAt }
