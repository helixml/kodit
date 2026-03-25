package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

// CreatePipelineParams holds the parameters for creating a new pipeline.
type CreatePipelineParams struct {
	Name  string
	Steps []StepParams
}

// StepParams describes a step to create within a pipeline.
type StepParams struct {
	Name      string
	Kind      string
	DependsOn []string
}

// UpdatePipelineParams holds the parameters for updating an existing pipeline.
type UpdatePipelineParams struct {
	Name  string
	Steps []StepParams
}

// PipelineDetail is an immutable read-only view combining a pipeline
// with its steps and dependency graph.
type PipelineDetail struct {
	pipeline     repository.Pipeline
	steps        []repository.Step
	dependencies []repository.StepDependency
}

// Pipeline returns the pipeline entity.
func (d PipelineDetail) Pipeline() repository.Pipeline { return d.pipeline }

// Steps returns the steps belonging to this pipeline.
func (d PipelineDetail) Steps() []repository.Step {
	result := make([]repository.Step, len(d.steps))
	copy(result, d.steps)
	return result
}

// Dependencies returns the step dependencies for this pipeline.
func (d PipelineDetail) Dependencies() []repository.StepDependency {
	result := make([]repository.StepDependency, len(d.dependencies))
	copy(result, d.dependencies)
	return result
}

// Pipeline provides CRUD operations for pipelines and their steps.
type Pipeline struct {
	repository.Collection[repository.Pipeline]
	pipelineStore     repository.PipelineStore
	stepStore         repository.StepStore
	pipelineStepStore repository.PipelineStepStore
	dependencyStore   repository.StepDependencyStore
	prescribedOps     task.PrescribedOperations
}

// NewPipeline creates a new Pipeline service.
func NewPipeline(
	pipelineStore repository.PipelineStore,
	stepStore repository.StepStore,
	pipelineStepStore repository.PipelineStepStore,
	dependencyStore repository.StepDependencyStore,
	prescribedOps task.PrescribedOperations,
) *Pipeline {
	return &Pipeline{
		Collection:        repository.NewCollection[repository.Pipeline](pipelineStore),
		pipelineStore:     pipelineStore,
		stepStore:         stepStore,
		pipelineStepStore: pipelineStepStore,
		dependencyStore:   dependencyStore,
		prescribedOps:     prescribedOps,
	}
}

// Initialise seeds the default and RAG pipelines if no pipelines exist yet.
func (s *Pipeline) Initialise(ctx context.Context) error {
	count, err := s.pipelineStore.Count(ctx)
	if err != nil {
		return fmt.Errorf("count pipelines: %w", err)
	}
	if count > 0 {
		return nil
	}

	// "default" pipeline — the full operation set from prescribedOps.
	defaultOps := s.prescribedOps.ScanAndIndexCommit()
	if _, err := s.Create(ctx, &CreatePipelineParams{
		Name:  "default",
		Steps: operationsToStepParams(defaultOps),
	}); err != nil {
		return fmt.Errorf("seed default pipeline: %w", err)
	}

	// "rag" pipeline — RAG-only subset (no enrichments).
	ragOps := task.RAGOnlyPrescribedOperations().ScanAndIndexCommit()
	if _, err := s.Create(ctx, &CreatePipelineParams{
		Name:  "rag",
		Steps: operationsToStepParams(ragOps),
	}); err != nil {
		return fmt.Errorf("seed rag pipeline: %w", err)
	}

	return nil
}

// operationsToStepParams converts an ordered slice of operations into step
// params with a linear dependency chain.
func operationsToStepParams(ops []task.Operation) []StepParams {
	steps := make([]StepParams, len(ops))
	for i, op := range ops {
		name := string(op)
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[idx+1:]
		}
		var dependsOn []string
		if i > 0 {
			dependsOn = []string{steps[i-1].Name}
		}
		steps[i] = StepParams{
			Name:      name,
			Kind:      string(op),
			DependsOn: dependsOn,
		}
	}
	return steps
}

// RequiredOperations returns all operations that handlers must support.
func (s *Pipeline) RequiredOperations() []task.Operation {
	return s.prescribedOps.All()
}

// FindSteps delegates to the step store for top-level step queries.
func (s *Pipeline) FindSteps(ctx context.Context, options ...repository.Option) ([]repository.Step, error) {
	return s.stepStore.Find(ctx, options...)
}

// FindStep returns a single step by ID.
func (s *Pipeline) FindStep(ctx context.Context, id int64) (repository.Step, error) {
	return s.stepStore.FindOne(ctx, repository.WithID(id))
}

// CountSteps delegates to the step store for counting steps.
func (s *Pipeline) CountSteps(ctx context.Context, options ...repository.Option) (int64, error) {
	return s.stepStore.Count(ctx, options...)
}

// StepDetail is an immutable read-only view of a step with its dependencies.
type StepDetail struct {
	step         repository.Step
	dependencies []repository.StepDependency
}

// Step returns the step entity.
func (d StepDetail) Step() repository.Step { return d.step }

// Dependencies returns the step dependencies.
func (d StepDetail) Dependencies() []repository.StepDependency {
	result := make([]repository.StepDependency, len(d.dependencies))
	copy(result, d.dependencies)
	return result
}

// DetailStep loads a step with its dependencies.
func (s *Pipeline) DetailStep(ctx context.Context, stepID int64) (StepDetail, error) {
	step, err := s.stepStore.FindOne(ctx, repository.WithID(stepID))
	if err != nil {
		return StepDetail{}, fmt.Errorf("find step: %w", err)
	}

	deps, err := s.dependencyStore.Find(ctx, repository.WithStepID(stepID))
	if err != nil {
		return StepDetail{}, fmt.Errorf("find dependencies: %w", err)
	}

	return StepDetail{
		step:         step,
		dependencies: deps,
	}, nil
}

// Create validates and persists a new pipeline with its steps and dependencies.
func (s *Pipeline) Create(ctx context.Context, params *CreatePipelineParams) (PipelineDetail, error) {
	if err := validatePipelineParams(params.Name, params.Steps); err != nil {
		return PipelineDetail{}, err
	}

	saved, err := s.pipelineStore.Save(ctx, repository.NewPipeline(params.Name))
	if err != nil {
		return PipelineDetail{}, fmt.Errorf("save pipeline: %w", err)
	}

	steps, deps, err := s.createSteps(ctx, saved.ID(), params.Steps)
	if err != nil {
		return PipelineDetail{}, err
	}

	return PipelineDetail{pipeline: saved, steps: steps, dependencies: deps}, nil
}

// Detail returns a pipeline with all its steps and dependencies.
func (s *Pipeline) Detail(ctx context.Context, id int64) (PipelineDetail, error) {
	pipeline, err := s.pipelineStore.FindOne(ctx, repository.WithID(id))
	if err != nil {
		return PipelineDetail{}, fmt.Errorf("find pipeline: %w", err)
	}

	steps, deps, err := s.loadStepsAndDependencies(ctx, pipeline.ID())
	if err != nil {
		return PipelineDetail{}, err
	}

	return PipelineDetail{pipeline: pipeline, steps: steps, dependencies: deps}, nil
}

// Update replaces all steps and dependencies for an existing pipeline.
func (s *Pipeline) Update(ctx context.Context, id int64, params *UpdatePipelineParams) (PipelineDetail, error) {
	if err := validatePipelineParams(params.Name, params.Steps); err != nil {
		return PipelineDetail{}, err
	}

	pipeline, err := s.pipelineStore.FindOne(ctx, repository.WithID(id))
	if err != nil {
		return PipelineDetail{}, fmt.Errorf("find pipeline: %w", err)
	}

	if err := s.deleteStepsForPipeline(ctx, pipeline.ID()); err != nil {
		return PipelineDetail{}, err
	}

	updated := repository.ReconstructPipeline(pipeline.ID(), params.Name, pipeline.CreatedAt(), pipeline.UpdatedAt())
	saved, err := s.pipelineStore.Save(ctx, updated)
	if err != nil {
		return PipelineDetail{}, fmt.Errorf("save pipeline: %w", err)
	}

	steps, deps, err := s.createSteps(ctx, saved.ID(), params.Steps)
	if err != nil {
		return PipelineDetail{}, err
	}

	return PipelineDetail{pipeline: saved, steps: steps, dependencies: deps}, nil
}

// Delete removes a pipeline and all associated steps and dependencies.
func (s *Pipeline) Delete(ctx context.Context, id int64) error {
	pipeline, err := s.pipelineStore.FindOne(ctx, repository.WithID(id))
	if err != nil {
		return fmt.Errorf("find pipeline: %w", err)
	}

	if err := s.deleteStepsForPipeline(ctx, pipeline.ID()); err != nil {
		return err
	}

	if err := s.pipelineStore.Delete(ctx, pipeline); err != nil {
		return fmt.Errorf("delete pipeline: %w", err)
	}

	return nil
}

// Operations loads the pipeline, topologically sorts its steps, and returns
// their Kind values as task.Operation. When pipelineID is nil, the first
// pipeline by ID is used as a default.
func (s *Pipeline) Operations(ctx context.Context, pipelineID *int64) ([]task.Operation, error) {
	var id int64
	if pipelineID != nil {
		id = *pipelineID
	} else {
		pipelines, err := s.pipelineStore.Find(ctx, repository.WithLimit(1), repository.WithOrderAsc("id"))
		if err != nil {
			return nil, fmt.Errorf("find default pipeline: %w", err)
		}
		if len(pipelines) == 0 {
			return nil, fmt.Errorf("no pipelines exist")
		}
		id = pipelines[0].ID()
	}

	detail, err := s.Detail(ctx, id)
	if err != nil {
		return nil, err
	}

	return topologicalSort(detail.Steps(), detail.Dependencies()), nil
}

// topologicalSort returns step kinds in dependency order (Kahn's algorithm).
func topologicalSort(steps []repository.Step, deps []repository.StepDependency) []task.Operation {
	idToStep := make(map[int64]repository.Step, len(steps))
	inDegree := make(map[int64]int, len(steps))
	dependents := make(map[int64][]int64, len(steps))

	for _, step := range steps {
		idToStep[step.ID()] = step
		inDegree[step.ID()] = 0
	}

	for _, dep := range deps {
		inDegree[dep.StepID()]++
		dependents[dep.DependsOnID()] = append(dependents[dep.DependsOnID()], dep.StepID())
	}

	queue := make([]int64, 0, len(steps))
	for _, step := range steps {
		if inDegree[step.ID()] == 0 {
			queue = append(queue, step.ID())
		}
	}

	operations := make([]task.Operation, 0, len(steps))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		operations = append(operations, task.Operation(idToStep[current].Kind()))
		for _, depID := range dependents[current] {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				queue = append(queue, depID)
			}
		}
	}

	return operations
}

// createSteps finds or creates steps by kind, then saves pipeline-step
// associations and dependencies. Steps are shared across pipelines.
func (s *Pipeline) createSteps(ctx context.Context, pipelineID int64, params []StepParams) ([]repository.Step, []repository.StepDependency, error) {
	nameToID := make(map[string]int64, len(params))
	steps := make([]repository.Step, 0, len(params))

	for _, sp := range params {
		step, err := s.findOrCreateStep(ctx, sp.Name, sp.Kind)
		if err != nil {
			return nil, nil, err
		}
		nameToID[sp.Name] = step.ID()
		steps = append(steps, step)

		_, err = s.pipelineStepStore.Save(ctx, repository.NewPipelineStep(pipelineID, step.ID()))
		if err != nil {
			return nil, nil, fmt.Errorf("save pipeline step for %q: %w", sp.Name, err)
		}
	}

	var deps []repository.StepDependency
	for _, sp := range params {
		stepID := nameToID[sp.Name]
		for _, depName := range sp.DependsOn {
			depID := nameToID[depName]
			dep, err := s.findOrCreateDependency(ctx, stepID, depID)
			if err != nil {
				return nil, nil, fmt.Errorf("dependency %q -> %q: %w", sp.Name, depName, err)
			}
			deps = append(deps, dep)
		}
	}

	return steps, deps, nil
}

// findOrCreateStep returns an existing step with the given kind, or creates one.
func (s *Pipeline) findOrCreateStep(ctx context.Context, name, kind string) (repository.Step, error) {
	existing, err := s.stepStore.FindOne(ctx, repository.WithKind(kind))
	if err == nil {
		return existing, nil
	}

	saved, err := s.stepStore.Save(ctx, repository.NewStep(name, kind))
	if err != nil {
		return repository.Step{}, fmt.Errorf("save step %q: %w", name, err)
	}
	return saved, nil
}

// findOrCreateDependency returns an existing dependency or creates one.
func (s *Pipeline) findOrCreateDependency(ctx context.Context, stepID, dependsOnID int64) (repository.StepDependency, error) {
	existing, err := s.dependencyStore.FindOne(ctx, repository.WithStepID(stepID), repository.WithDependsOnID(dependsOnID))
	if err == nil {
		return existing, nil
	}

	saved, err := s.dependencyStore.Save(ctx, repository.NewStepDependency(stepID, dependsOnID))
	if err != nil {
		return repository.StepDependency{}, fmt.Errorf("save dependency: %w", err)
	}
	return saved, nil
}

// loadStepsAndDependencies fetches steps and their dependencies for a pipeline.
func (s *Pipeline) loadStepsAndDependencies(ctx context.Context, pipelineID int64) ([]repository.Step, []repository.StepDependency, error) {
	associations, err := s.pipelineStepStore.Find(ctx, repository.WithPipelineID(pipelineID))
	if err != nil {
		return nil, nil, fmt.Errorf("find pipeline steps: %w", err)
	}

	if len(associations) == 0 {
		return nil, nil, nil
	}

	stepIDs := make([]int64, len(associations))
	for i, a := range associations {
		stepIDs[i] = a.StepID()
	}

	steps, err := s.stepStore.Find(ctx, repository.WithIDIn(stepIDs))
	if err != nil {
		return nil, nil, fmt.Errorf("find steps: %w", err)
	}

	deps, err := s.dependencyStore.Find(ctx, repository.WithStepIDIn(stepIDs))
	if err != nil {
		return nil, nil, fmt.Errorf("find dependencies: %w", err)
	}

	return steps, deps, nil
}

// deleteStepsForPipeline removes pipeline-step associations for a pipeline,
// then deletes any orphaned steps (steps no longer referenced by any pipeline).
func (s *Pipeline) deleteStepsForPipeline(ctx context.Context, pipelineID int64) error {
	associations, err := s.pipelineStepStore.Find(ctx, repository.WithPipelineID(pipelineID))
	if err != nil {
		return fmt.Errorf("find pipeline steps: %w", err)
	}

	if len(associations) == 0 {
		return nil
	}

	stepIDs := make([]int64, len(associations))
	for i, a := range associations {
		stepIDs[i] = a.StepID()
	}

	// Remove the pipeline-step associations.
	if err := s.pipelineStepStore.DeleteBy(ctx, repository.WithPipelineID(pipelineID)); err != nil {
		return fmt.Errorf("delete pipeline steps: %w", err)
	}

	// Delete orphaned steps — those no longer associated with any pipeline.
	for _, stepID := range stepIDs {
		remaining, err := s.pipelineStepStore.Find(ctx, repository.WithStepID(stepID))
		if err != nil {
			return fmt.Errorf("check step %d associations: %w", stepID, err)
		}
		if len(remaining) == 0 {
			if err := s.stepStore.DeleteBy(ctx, repository.WithID(stepID)); err != nil {
				return fmt.Errorf("delete orphaned step %d: %w", stepID, err)
			}
		}
	}

	return nil
}

// validatePipelineParams checks that pipeline parameters are valid.
func validatePipelineParams(name string, steps []StepParams) error {
	if name == "" {
		return fmt.Errorf("pipeline name is required")
	}
	if len(steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}

	names := make(map[string]bool, len(steps))
	for _, sp := range steps {
		if sp.Name == "" {
			return fmt.Errorf("step name is required")
		}
		if sp.Kind == "" {
			return fmt.Errorf("step kind is required for step %q", sp.Name)
		}
		if names[sp.Name] {
			return fmt.Errorf("duplicate step name %q", sp.Name)
		}
		names[sp.Name] = true
	}

	for _, sp := range steps {
		for _, dep := range sp.DependsOn {
			if !names[dep] {
				return fmt.Errorf("step %q depends on unknown step %q", sp.Name, dep)
			}
		}
	}

	if err := detectCycle(steps); err != nil {
		return err
	}

	return nil
}

// detectCycle checks for circular dependencies using topological sort.
func detectCycle(steps []StepParams) error {
	inDegree := make(map[string]int, len(steps))
	dependents := make(map[string][]string, len(steps))

	for _, sp := range steps {
		if _, ok := inDegree[sp.Name]; !ok {
			inDegree[sp.Name] = 0
		}
		for _, dep := range sp.DependsOn {
			inDegree[sp.Name]++
			dependents[dep] = append(dependents[dep], sp.Name)
		}
	}

	queue := make([]string, 0, len(steps))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		visited++
		for _, dep := range dependents[current] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if visited != len(steps) {
		return fmt.Errorf("circular dependency detected among steps")
	}

	return nil
}
