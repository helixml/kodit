package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/testdb"
)

func newPipelineService(t *testing.T) *Pipeline {
	t.Helper()
	db := testdb.New(t)
	return NewPipeline(
		persistence.NewPipelineStore(db),
		persistence.NewStepStore(db),
		persistence.NewPipelineStepStore(db),
		persistence.NewStepDependencyStore(db),
	)
}

func TestPipeline_Create(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	detail, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "build-pipeline",
		Steps: []StepParams{
			{Name: "clone", Kind: "git"},
			{Name: "test", Kind: "shell", DependsOn: []string{"clone"}},
			{Name: "deploy", Kind: "shell", DependsOn: []string{"test"}},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if detail.Pipeline().Name() != "build-pipeline" {
		t.Errorf("expected name %q, got %q", "build-pipeline", detail.Pipeline().Name())
	}
	if len(detail.Steps()) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(detail.Steps()))
	}
	if len(detail.Dependencies()) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(detail.Dependencies()))
	}
}

func TestPipeline_Detail(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "my-pipeline",
		Steps: []StepParams{
			{Name: "fetch", Kind: "git"},
			{Name: "build", Kind: "shell", DependsOn: []string{"fetch"}},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	detail, err := svc.Detail(ctx, created.Pipeline().ID())
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}

	if detail.Pipeline().ID() != created.Pipeline().ID() {
		t.Errorf("expected pipeline ID %d, got %d", created.Pipeline().ID(), detail.Pipeline().ID())
	}
	if len(detail.Steps()) != 2 {
		t.Errorf("expected 2 steps, got %d", len(detail.Steps()))
	}
	if len(detail.Dependencies()) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(detail.Dependencies()))
	}
}

func TestPipeline_Update(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "original",
		Steps: []StepParams{
			{Name: "step-a", Kind: "shell"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Pipeline().ID(), &UpdatePipelineParams{
		Name: "updated",
		Steps: []StepParams{
			{Name: "step-x", Kind: "git"},
			{Name: "step-y", Kind: "shell", DependsOn: []string{"step-x"}},
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Pipeline().Name() != "updated" {
		t.Errorf("expected name %q, got %q", "updated", updated.Pipeline().Name())
	}
	if updated.Pipeline().ID() != created.Pipeline().ID() {
		t.Errorf("expected same pipeline ID %d, got %d", created.Pipeline().ID(), updated.Pipeline().ID())
	}
	if len(updated.Steps()) != 2 {
		t.Errorf("expected 2 steps, got %d", len(updated.Steps()))
	}
	if len(updated.Dependencies()) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(updated.Dependencies()))
	}

	// Verify old steps are gone by loading detail
	detail, err := svc.Detail(ctx, created.Pipeline().ID())
	if err != nil {
		t.Fatalf("Detail after update: %v", err)
	}
	for _, step := range detail.Steps() {
		if step.Name() == "step-a" {
			t.Error("old step 'step-a' should have been deleted")
		}
	}
}

func TestPipeline_Delete(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "to-delete",
		Steps: []StepParams{
			{Name: "step-1", Kind: "shell"},
			{Name: "step-2", Kind: "shell", DependsOn: []string{"step-1"}},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(ctx, created.Pipeline().ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Pipeline should be gone
	_, err = svc.Detail(ctx, created.Pipeline().ID())
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}

	// Steps should be gone
	count, err := svc.Find(ctx)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(count) != 0 {
		t.Errorf("expected 0 pipelines, got %d", len(count))
	}
}

func TestPipeline_ValidationEmptyName(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "",
		Steps: []StepParams{
			{Name: "step", Kind: "shell"},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
}

func TestPipeline_ValidationNoSteps(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, &CreatePipelineParams{
		Name:  "no-steps",
		Steps: nil,
	})
	if err == nil {
		t.Fatal("expected validation error for no steps")
	}
}

func TestPipeline_ValidationDuplicateStepNames(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "dup-names",
		Steps: []StepParams{
			{Name: "step", Kind: "shell"},
			{Name: "step", Kind: "git"},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for duplicate step names")
	}
}

func TestPipeline_ValidationMissingDependency(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "missing-dep",
		Steps: []StepParams{
			{Name: "step", Kind: "shell", DependsOn: []string{"nonexistent"}},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for missing dependency reference")
	}
}

func TestPipeline_ValidationCircularDependency(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "circular",
		Steps: []StepParams{
			{Name: "a", Kind: "shell", DependsOn: []string{"b"}},
			{Name: "b", Kind: "shell", DependsOn: []string{"a"}},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for circular dependency")
	}
}

func TestPipeline_FindAndCount(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	for _, name := range []string{"pipeline-a", "pipeline-b"} {
		_, err := svc.Create(ctx, &CreatePipelineParams{
			Name:  name,
			Steps: []StepParams{{Name: "step", Kind: "shell"}},
		})
		if err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	pipelines, err := svc.Find(ctx)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(pipelines) != 2 {
		t.Errorf("expected 2 pipelines, got %d", len(pipelines))
	}

	count, err := svc.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	// Get by ID
	got, err := svc.Get(ctx, repository.WithName("pipeline-a"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name() != "pipeline-a" {
		t.Errorf("expected %q, got %q", "pipeline-a", got.Name())
	}
}
