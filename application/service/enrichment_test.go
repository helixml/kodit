package service

import (
	"context"
	"strconv"
	"testing"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/sourcelocation"
)

// Recording fakes for search stores — genuine fakes because real stores
// need pgvector/ParadeDB, and we need to verify DeleteBy was called.

type recordingBM25Store struct {
	deleteCalled bool
	deleteOpts   []repository.Option
}

func (r *recordingBM25Store) Index(_ context.Context, _ search.IndexRequest) error { return nil }
func (r *recordingBM25Store) Find(_ context.Context, _ ...repository.Option) ([]search.Result, error) {
	return nil, nil
}
func (r *recordingBM25Store) ExistingIDs(_ context.Context, _ []string) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}
func (r *recordingBM25Store) DeleteBy(_ context.Context, opts ...repository.Option) error {
	r.deleteCalled = true
	r.deleteOpts = opts
	return nil
}

type recordingEmbeddingStore struct {
	deleteCalled bool
	deleteOpts   []repository.Option
}

func (r *recordingEmbeddingStore) SaveAll(_ context.Context, _ []search.Embedding) error {
	return nil
}
func (r *recordingEmbeddingStore) Find(_ context.Context, _ ...repository.Option) ([]search.Embedding, error) {
	return nil, nil
}
func (r *recordingEmbeddingStore) Search(_ context.Context, _ ...repository.Option) ([]search.Result, error) {
	return nil, nil
}
func (r *recordingEmbeddingStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return false, nil
}
func (r *recordingEmbeddingStore) DeleteBy(_ context.Context, opts ...repository.Option) error {
	r.deleteCalled = true
	r.deleteOpts = opts
	return nil
}

func TestEnrichment_SourceLocations(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	e := enrichment.NewEnrichment(
		enrichment.TypeDevelopment,
		enrichment.SubtypeSnippet,
		enrichment.EntityTypeSnippet,
		"func main() {}",
	)
	saved, err := stores.enrichments.Save(ctx, e)
	if err != nil {
		t.Fatalf("save enrichment: %v", err)
	}

	lr := sourcelocation.New(saved.ID(), 10, 25)
	_, err = stores.lineRanges.Save(ctx, lr)
	if err != nil {
		t.Fatalf("save line range: %v", err)
	}

	svc := NewEnrichment(stores.enrichments, nil, nil, nil, nil, nil, stores.lineRanges)

	result, err := svc.SourceLocations(ctx, []int64{saved.ID()})
	if err != nil {
		t.Fatalf("SourceLocations: %v", err)
	}

	idStr := strconv.FormatInt(saved.ID(), 10)
	got, ok := result[idStr]
	if !ok {
		t.Fatalf("missing line range for enrichment ID %s; keys: %v", idStr, result)
	}
	if got.StartLine() != 10 {
		t.Errorf("StartLine = %d, want 10", got.StartLine())
	}
	if got.EndLine() != 25 {
		t.Errorf("EndLine = %d, want 25", got.EndLine())
	}
}

func TestEnrichment_SourceLocations_EmptyIDs(t *testing.T) {
	svc := NewEnrichment(nil, nil, nil, nil, nil, nil, nil)

	result, err := svc.SourceLocations(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestEnrichment_List_WithTypeFilter(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	e1 := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code1")
	e2 := enrichment.NewEnrichment(enrichment.TypeUsage, enrichment.SubtypeCookbook, enrichment.EntityTypeSnippet, "cookbook1")
	if _, err := stores.enrichments.Save(ctx, e1); err != nil {
		t.Fatalf("save e1: %v", err)
	}
	if _, err := stores.enrichments.Save(ctx, e2); err != nil {
		t.Fatalf("save e2: %v", err)
	}

	svc := NewEnrichment(stores.enrichments, nil, nil, nil, nil, nil, nil)
	typ := enrichment.TypeDevelopment
	results, err := svc.List(ctx, &EnrichmentListParams{Type: &typ})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Type() != enrichment.TypeDevelopment {
		t.Errorf("expected TypeDevelopment, got %s", results[0].Type())
	}
}

func TestEnrichment_List_WithPagination(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		e := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code")
		if _, err := stores.enrichments.Save(ctx, e); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	svc := NewEnrichment(stores.enrichments, nil, nil, nil, nil, nil, nil)
	results, err := svc.List(ctx, &EnrichmentListParams{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestEnrichment_List_NilParams(t *testing.T) {
	svc := NewEnrichment(nil, nil, nil, nil, nil, nil, nil)

	results, err := svc.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(results))
	}
}

func TestEnrichment_Count_WithFilter(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	e1 := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code1")
	e2 := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code2")
	e3 := enrichment.NewEnrichment(enrichment.TypeUsage, enrichment.SubtypeCookbook, enrichment.EntityTypeSnippet, "cookbook1")
	for _, e := range []enrichment.Enrichment{e1, e2, e3} {
		if _, err := stores.enrichments.Save(ctx, e); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	svc := NewEnrichment(stores.enrichments, nil, nil, nil, nil, nil, nil)
	typ := enrichment.TypeDevelopment
	count, err := svc.Count(ctx, &EnrichmentListParams{Type: &typ})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestEnrichment_DeleteBy_CleansUpSearchIndexes(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	e := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code1")
	saved, err := stores.enrichments.Save(ctx, e)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	bm25 := &recordingBM25Store{}
	codeEmb := &recordingEmbeddingStore{}
	textEmb := &recordingEmbeddingStore{}
	visionEmb := &recordingEmbeddingStore{}

	svc := NewEnrichment(stores.enrichments, nil, bm25, codeEmb, textEmb, visionEmb, nil)
	if err := svc.DeleteBy(ctx, repository.WithID(saved.ID())); err != nil {
		t.Fatalf("DeleteBy: %v", err)
	}

	if !bm25.deleteCalled {
		t.Error("expected bm25Store.DeleteBy to be called")
	}
	if !codeEmb.deleteCalled {
		t.Error("expected codeEmbeddingStore.DeleteBy to be called")
	}
	if !textEmb.deleteCalled {
		t.Error("expected textEmbeddingStore.DeleteBy to be called")
	}
	if !visionEmb.deleteCalled {
		t.Error("expected visionEmbeddingStore.DeleteBy to be called")
	}

	remaining, err := stores.enrichments.Find(ctx, repository.WithID(saved.ID()))
	if err != nil {
		t.Fatalf("find after delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected enrichment to be deleted, got %d", len(remaining))
	}
}

func TestEnrichment_DeleteBy_NilStores(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	e := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code1")
	saved, err := stores.enrichments.Save(ctx, e)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// Vision store is always required; BM25, code, and text stores may be nil.
	visionEmb := &recordingEmbeddingStore{}
	svc := NewEnrichment(stores.enrichments, nil, nil, nil, nil, visionEmb, nil)
	if err := svc.DeleteBy(ctx, repository.WithID(saved.ID())); err != nil {
		t.Fatalf("DeleteBy: %v", err)
	}

	remaining, err := stores.enrichments.Find(ctx, repository.WithID(saved.ID()))
	if err != nil {
		t.Fatalf("find after delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected enrichment to be deleted, got %d", len(remaining))
	}
}

func TestEnrichment_RelatedEnrichments(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	parent := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "parent code")
	savedParent, err := stores.enrichments.Save(ctx, parent)
	if err != nil {
		t.Fatalf("save parent: %v", err)
	}

	related := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippetSummary, enrichment.EntityTypeSnippet, "summary of parent")
	savedRelated, err := stores.enrichments.Save(ctx, related)
	if err != nil {
		t.Fatalf("save related: %v", err)
	}

	parentIDStr := strconv.FormatInt(savedParent.ID(), 10)
	assoc := enrichment.NewAssociation(savedRelated.ID(), parentIDStr, enrichment.EntityTypeSnippet)
	if _, err := stores.associations.Save(ctx, assoc); err != nil {
		t.Fatalf("save association: %v", err)
	}

	svc := NewEnrichment(stores.enrichments, stores.associations, nil, nil, nil, nil, nil)
	result, err := svc.RelatedEnrichments(ctx, []int64{savedParent.ID()})
	if err != nil {
		t.Fatalf("RelatedEnrichments: %v", err)
	}

	got, ok := result[parentIDStr]
	if !ok {
		t.Fatalf("expected entry for parent ID %s, got keys: %v", parentIDStr, result)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 related enrichment, got %d", len(got))
	}
	if got[0].ID() != savedRelated.ID() {
		t.Errorf("expected related ID %d, got %d", savedRelated.ID(), got[0].ID())
	}
}

func TestEnrichment_RelatedEnrichments_EmptyIDs(t *testing.T) {
	svc := NewEnrichment(nil, nil, nil, nil, nil, nil, nil)

	result, err := svc.RelatedEnrichments(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestEnrichment_SourceFiles(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	e := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code")
	saved, err := stores.enrichments.Save(ctx, e)
	if err != nil {
		t.Fatalf("save enrichment: %v", err)
	}

	assoc1 := enrichment.NewAssociation(saved.ID(), "100", enrichment.EntityTypeFile)
	assoc2 := enrichment.NewAssociation(saved.ID(), "200", enrichment.EntityTypeFile)
	if _, err := stores.associations.Save(ctx, assoc1); err != nil {
		t.Fatalf("save assoc1: %v", err)
	}
	if _, err := stores.associations.Save(ctx, assoc2); err != nil {
		t.Fatalf("save assoc2: %v", err)
	}

	svc := NewEnrichment(stores.enrichments, stores.associations, nil, nil, nil, nil, nil)
	result, err := svc.SourceFiles(ctx, []int64{saved.ID()})
	if err != nil {
		t.Fatalf("SourceFiles: %v", err)
	}

	key := strconv.FormatInt(saved.ID(), 10)
	files, ok := result[key]
	if !ok {
		t.Fatalf("expected entry for enrichment ID %s", key)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 file IDs, got %d", len(files))
	}
}

func TestEnrichment_SourceFiles_EmptyIDs(t *testing.T) {
	svc := NewEnrichment(nil, nil, nil, nil, nil, nil, nil)

	result, err := svc.SourceFiles(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestEnrichment_RepositoryIDs(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	e := enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code")
	saved, err := stores.enrichments.Save(ctx, e)
	if err != nil {
		t.Fatalf("save enrichment: %v", err)
	}

	assoc := enrichment.NewAssociation(saved.ID(), "42", enrichment.EntityTypeRepository)
	if _, err := stores.associations.Save(ctx, assoc); err != nil {
		t.Fatalf("save association: %v", err)
	}

	svc := NewEnrichment(stores.enrichments, stores.associations, nil, nil, nil, nil, nil)
	result, err := svc.RepositoryIDs(ctx, []int64{saved.ID()})
	if err != nil {
		t.Fatalf("RepositoryIDs: %v", err)
	}

	key := strconv.FormatInt(saved.ID(), 10)
	repoID, ok := result[key]
	if !ok {
		t.Fatalf("expected entry for enrichment ID %s", key)
	}
	if repoID != 42 {
		t.Errorf("expected repo ID 42, got %d", repoID)
	}
}

func TestEnrichment_RepositoryIDs_EmptyIDs(t *testing.T) {
	svc := NewEnrichment(nil, nil, nil, nil, nil, nil, nil)

	result, err := svc.RepositoryIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}
