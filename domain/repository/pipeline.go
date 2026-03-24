package repository

import "time"

// Pipeline represents a processing pipeline for a repository.
type Pipeline struct {
	id        int64
	repoID    int64
	steps     []Step
	createdAt time.Time
	updatedAt time.Time
}

// NewPipeline creates a new Pipeline for the given repository.
func NewPipeline(repoID int64, steps []Step) Pipeline {
	now := time.Now()
	return Pipeline{
		repoID:    repoID,
		steps:     steps,
		createdAt: now,
		updatedAt: now,
	}
}

// ReconstructPipeline rebuilds a Pipeline from persisted data.
func ReconstructPipeline(id, repoID int64, steps []Step, createdAt, updatedAt time.Time) Pipeline {
	return Pipeline{
		id:        id,
		repoID:    repoID,
		steps:     steps,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// ID returns the pipeline identifier.
func (p Pipeline) ID() int64 { return p.id }

// RepoID returns the associated repository identifier.
func (p Pipeline) RepoID() int64 { return p.repoID }

// Steps returns the pipeline steps.
func (p Pipeline) Steps() []Step { return p.steps }

// CreatedAt returns the creation timestamp.
func (p Pipeline) CreatedAt() time.Time { return p.createdAt }

// UpdatedAt returns the last update timestamp.
func (p Pipeline) UpdatedAt() time.Time { return p.updatedAt }

// WithSteps returns a copy with the given steps.
func (p Pipeline) WithSteps(steps []Step) Pipeline {
	p.steps = steps
	p.updatedAt = time.Now()
	return p
}

// Step represents a single step in a pipeline.
type Step struct {
	id           int64
	kind         string
	dependencies []int64
	createdAt    time.Time
	updatedAt    time.Time
}

// NewStep creates a new Step with the given kind and dependency step IDs.
func NewStep(kind string, dependencies []int64) Step {
	now := time.Now()
	return Step{
		kind:         kind,
		dependencies: dependencies,
		createdAt:    now,
		updatedAt:    now,
	}
}

// ReconstructStep rebuilds a Step from persisted data.
func ReconstructStep(id int64, kind string, dependencies []int64, createdAt, updatedAt time.Time) Step {
	return Step{
		id:           id,
		kind:         kind,
		dependencies: dependencies,
		createdAt:    createdAt,
		updatedAt:    updatedAt,
	}
}

// ID returns the step identifier.
func (s Step) ID() int64 { return s.id }

// Kind returns the step kind.
func (s Step) Kind() string { return s.kind }

// Dependencies returns the IDs of steps this step depends on.
func (s Step) Dependencies() []int64 { return s.dependencies }

// CreatedAt returns the creation timestamp.
func (s Step) CreatedAt() time.Time { return s.createdAt }

// UpdatedAt returns the last update timestamp.
func (s Step) UpdatedAt() time.Time { return s.updatedAt }
