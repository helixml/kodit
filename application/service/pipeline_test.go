package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
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
		task.FullPrescribedOperations(),
	)
}

func TestPipeline_Create(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	detail, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "build-pipeline",
		Steps: []StepParams{
			{Name: "clone", Kind: "git.clone"},
			{Name: "test", Kind: "shell.test", DependsOn: []string{"clone"}},
			{Name: "deploy", Kind: "shell.deploy", DependsOn: []string{"test"}},
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
			{Name: "fetch", Kind: "git.fetch"},
			{Name: "build", Kind: "shell.build", DependsOn: []string{"fetch"}},
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
			{Name: "step-a", Kind: "shell.a"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Pipeline().ID(), &UpdatePipelineParams{
		Name: "updated",
		Steps: []StepParams{
			{Name: "step-x", Kind: "git.x"},
			{Name: "step-y", Kind: "shell.y", DependsOn: []string{"step-x"}},
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
			{Name: "step-1", Kind: "shell.one"},
			{Name: "step-2", Kind: "shell.two", DependsOn: []string{"step-1"}},
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

func TestPipeline_Operations(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	detail, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "ordered-pipeline",
		Steps: []StepParams{
			{Name: string(task.OperationScanCommit), Kind: "internal"},
			{Name: string(task.OperationExtractSnippetsForCommit), Kind: "internal", DependsOn: []string{string(task.OperationScanCommit)}},
			{Name: string(task.OperationCreateBM25IndexForCommit), Kind: "internal", DependsOn: []string{string(task.OperationExtractSnippetsForCommit)}},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	pipelineID := detail.Pipeline().ID()
	ops, err := svc.Operations(ctx, pipelineID)
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}

	if len(ops) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(ops))
	}
	if ops[0] != task.OperationScanCommit {
		t.Errorf("expected first operation %q, got %q", task.OperationScanCommit, ops[0])
	}
	if ops[1] != task.OperationExtractSnippetsForCommit {
		t.Errorf("expected second operation %q, got %q", task.OperationExtractSnippetsForCommit, ops[1])
	}
	if ops[2] != task.OperationCreateBM25IndexForCommit {
		t.Errorf("expected third operation %q, got %q", task.OperationCreateBM25IndexForCommit, ops[2])
	}
}

func TestPipeline_Operations_NoDependencies(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	detail, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "parallel-pipeline",
		Steps: []StepParams{
			{Name: string(task.OperationScanCommit), Kind: "internal"},
			{Name: string(task.OperationExtractSnippetsForCommit), Kind: "internal"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	pipelineID := detail.Pipeline().ID()
	ops, err := svc.Operations(ctx, pipelineID)
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}

	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}

	// Without dependencies, steps are returned in insertion order.
	kinds := map[task.Operation]bool{ops[0]: true, ops[1]: true}
	if !kinds[task.OperationScanCommit] {
		t.Errorf("expected %q in operations", task.OperationScanCommit)
	}
	if !kinds[task.OperationExtractSnippetsForCommit] {
		t.Errorf("expected %q in operations", task.OperationExtractSnippetsForCommit)
	}
}

func TestPipeline_DefaultID(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	if err := svc.Initialise(ctx); err != nil {
		t.Fatalf("Initialise: %v", err)
	}

	id, err := svc.DefaultID(ctx)
	if err != nil {
		t.Fatalf("DefaultID: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero default pipeline ID")
	}
}

func TestPipeline_Operations_DefaultPipeline(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	if err := svc.Initialise(ctx); err != nil {
		t.Fatalf("Initialise: %v", err)
	}

	defaultID, err := svc.DefaultID(ctx)
	if err != nil {
		t.Fatalf("DefaultID: %v", err)
	}

	ops, err := svc.Operations(ctx, defaultID)
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}

	if len(ops) == 0 {
		t.Fatal("expected at least one operation from the default pipeline")
	}
	if ops[0] != task.OperationScanCommit {
		t.Errorf("expected first operation %q, got %q", task.OperationScanCommit, ops[0])
	}
}

func TestPipeline_Initialise(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	if err := svc.Initialise(ctx); err != nil {
		t.Fatalf("Initialise: %v", err)
	}

	pipelines, err := svc.Find(ctx, repository.WithOrderAsc("id"))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}
	if pipelines[0].Name() != "default" {
		t.Errorf("expected first pipeline %q, got %q", "default", pipelines[0].Name())
	}
	if pipelines[1].Name() != "rag" {
		t.Errorf("expected second pipeline %q, got %q", "rag", pipelines[1].Name())
	}

	// RAG pipeline should have fewer steps than default.
	defaultDetail, err := svc.Detail(ctx, pipelines[0].ID())
	if err != nil {
		t.Fatalf("Detail default: %v", err)
	}
	ragDetail, err := svc.Detail(ctx, pipelines[1].ID())
	if err != nil {
		t.Fatalf("Detail rag: %v", err)
	}
	if len(ragDetail.Steps()) >= len(defaultDetail.Steps()) {
		t.Errorf("expected rag pipeline (%d steps) to have fewer steps than default (%d steps)",
			len(ragDetail.Steps()), len(defaultDetail.Steps()))
	}

	// Calling Initialise again should be idempotent.
	if err := svc.Initialise(ctx); err != nil {
		t.Fatalf("Initialise (second call): %v", err)
	}

	pipelines, err = svc.Find(ctx)
	if err != nil {
		t.Fatalf("Find after second Initialise: %v", err)
	}
	if len(pipelines) != 2 {
		t.Errorf("expected 2 pipelines after idempotent call, got %d", len(pipelines))
	}
}

func TestPipeline_JoinType_RoundTrip(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	detail, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "join-pipeline",
		Steps: []StepParams{
			{Name: "scan", Kind: "internal"},
			{Name: "extract", Kind: "internal", DependsOn: []string{"scan"}},
			{Name: "index", Kind: "internal", DependsOn: []string{"scan", "extract"}, JoinType: "any"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify associations from Create.
	assocs := detail.Associations()
	if len(assocs) != 3 {
		t.Fatalf("expected 3 associations, got %d", len(assocs))
	}

	joinByStep := make(map[int64]string, len(assocs))
	for _, a := range assocs {
		joinByStep[a.StepID()] = a.JoinType()
	}

	for _, s := range detail.Steps() {
		jt := joinByStep[s.ID()]
		switch s.Name() {
		case "scan", "extract":
			if jt != "all" {
				t.Errorf("step %q: expected join_type \"all\", got %q", s.Name(), jt)
			}
		case "index":
			if jt != "any" {
				t.Errorf("step %q: expected join_type \"any\", got %q", s.Name(), jt)
			}
		}
	}

	// Verify round-trip through Detail.
	loaded, err := svc.Detail(ctx, detail.Pipeline().ID())
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}

	loadedAssocs := loaded.Associations()
	if len(loadedAssocs) != 3 {
		t.Fatalf("expected 3 associations from Detail, got %d", len(loadedAssocs))
	}

	joinByStep = make(map[int64]string, len(loadedAssocs))
	for _, a := range loadedAssocs {
		joinByStep[a.StepID()] = a.JoinType()
	}

	for _, s := range loaded.Steps() {
		jt := joinByStep[s.ID()]
		switch s.Name() {
		case "scan", "extract":
			if jt != "all" {
				t.Errorf("Detail step %q: expected join_type \"all\", got %q", s.Name(), jt)
			}
		case "index":
			if jt != "any" {
				t.Errorf("Detail step %q: expected join_type \"any\", got %q", s.Name(), jt)
			}
		}
	}
}

func TestPipeline_JoinType_DefaultIsAll(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	detail, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "default-join",
		Steps: []StepParams{
			{Name: "step-a", Kind: "shell"},
			{Name: "step-b", Kind: "shell", DependsOn: []string{"step-a"}},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, a := range detail.Associations() {
		if a.JoinType() != "all" {
			t.Errorf("expected default join_type \"all\", got %q for step %d", a.JoinType(), a.StepID())
		}
	}
}

func TestPipeline_JoinType_ValidationRejectsInvalid(t *testing.T) {
	svc := newPipelineService(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, &CreatePipelineParams{
		Name: "bad-join",
		Steps: []StepParams{
			{Name: "step-a", Kind: "shell", JoinType: "maybe"},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for invalid join_type")
	}
}

func TestPipeline_Initialise_Reconciles(t *testing.T) {
	db := testdb.New(t)
	pipelineStore := persistence.NewPipelineStore(db)
	stepStore := persistence.NewStepStore(db)
	pipelineStepStore := persistence.NewPipelineStepStore(db)
	depStore := persistence.NewStepDependencyStore(db)
	ctx := context.Background()

	// First boot: seed with RAG-only operations (smaller set).
	svc1 := NewPipeline(pipelineStore, stepStore, pipelineStepStore, depStore,
		task.RAGOnlyPrescribedOperations())
	if err := svc1.Initialise(ctx); err != nil {
		t.Fatalf("Initialise (v1): %v", err)
	}

	defaultID, err := svc1.DefaultID(ctx)
	if err != nil {
		t.Fatalf("DefaultID: %v", err)
	}
	v1Detail, err := svc1.Detail(ctx, defaultID)
	if err != nil {
		t.Fatalf("Detail v1: %v", err)
	}
	v1StepCount := len(v1Detail.Steps())

	// Second boot: upgrade to full operations (larger set, includes enrichments).
	svc2 := NewPipeline(pipelineStore, stepStore, pipelineStepStore, depStore,
		task.FullPrescribedOperations())
	if err := svc2.Initialise(ctx); err != nil {
		t.Fatalf("Initialise (v2): %v", err)
	}

	// The default pipeline should now have more steps.
	v2Detail, err := svc2.Detail(ctx, defaultID)
	if err != nil {
		t.Fatalf("Detail v2: %v", err)
	}
	if len(v2Detail.Steps()) <= v1StepCount {
		t.Errorf("expected more steps after reconciliation: v1=%d, v2=%d",
			v1StepCount, len(v2Detail.Steps()))
	}

	// Still exactly 2 pipelines.
	pipelines, err := svc2.Find(ctx)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(pipelines) != 2 {
		t.Errorf("expected 2 pipelines, got %d", len(pipelines))
	}
}

func TestPipeline_RequiredOperations(t *testing.T) {
	svc := newPipelineService(t)
	ops := svc.RequiredOperations()

	if len(ops) == 0 {
		t.Fatal("expected at least one required operation")
	}

	expected := task.FullPrescribedOperations().All()
	if len(ops) != len(expected) {
		t.Errorf("expected %d operations, got %d", len(expected), len(ops))
	}
}
