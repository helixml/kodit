package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// fakeEmbedder implements search.Embedder for testing.
type fakeEmbedder struct {
	vectors [][]float64
	err     error
}

func (f fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	if f.err != nil {
		return nil, f.err
	}
	result := make([][]float64, len(texts))
	for i := range texts {
		if i < len(f.vectors) {
			result[i] = f.vectors[i]
		} else {
			result[i] = f.vectors[0]
		}
	}
	return result, nil
}

// fakeEmbeddingStore implements search.EmbeddingStore for testing.
type fakeEmbeddingStore struct {
	results []search.Result
	err     error
}

func (f fakeEmbeddingStore) SaveAll(_ context.Context, _ []search.Embedding) error { return nil }
func (f fakeEmbeddingStore) Find(_ context.Context, _ ...repository.Option) ([]search.Embedding, error) {
	return nil, nil
}
func (f fakeEmbeddingStore) Search(_ context.Context, _ ...repository.Option) ([]search.Result, error) {
	return f.results, f.err
}
func (f fakeEmbeddingStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return false, nil
}
func (f fakeEmbeddingStore) DeleteBy(_ context.Context, _ ...repository.Option) error { return nil }

// fakeBM25Store implements search.BM25Store for testing.
type fakeBM25Store struct {
	resultsByKeyword map[string][]search.Result
	err              error
}

func (f fakeBM25Store) Index(_ context.Context, _ search.IndexRequest) error { return nil }
func (f fakeBM25Store) Find(_ context.Context, opts ...repository.Option) ([]search.Result, error) {
	if f.err != nil {
		return nil, f.err
	}
	q := repository.Build(opts...)
	query, _ := search.QueryFrom(q)
	return f.resultsByKeyword[query], nil
}
func (f fakeBM25Store) DeleteBy(_ context.Context, _ ...repository.Option) error { return nil }

// fakeEnrichmentStore implements enrichment.EnrichmentStore for testing.
type fakeEnrichmentStore struct {
	enrichments []enrichment.Enrichment
}

func (f fakeEnrichmentStore) Find(_ context.Context, _ ...repository.Option) ([]enrichment.Enrichment, error) {
	return f.enrichments, nil
}
func (f fakeEnrichmentStore) FindOne(_ context.Context, _ ...repository.Option) (enrichment.Enrichment, error) {
	if len(f.enrichments) > 0 {
		return f.enrichments[0], nil
	}
	return enrichment.Enrichment{}, nil
}
func (f fakeEnrichmentStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	return int64(len(f.enrichments)), nil
}
func (f fakeEnrichmentStore) Save(_ context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error) {
	return e, nil
}
func (f fakeEnrichmentStore) Delete(_ context.Context, _ enrichment.Enrichment) error  { return nil }
func (f fakeEnrichmentStore) DeleteBy(_ context.Context, _ ...repository.Option) error { return nil }

func TestSearch_EmbeddingFailure_ReturnsError(t *testing.T) {
	embedErr := errors.New("model unavailable")
	svc := NewSearch(
		fakeEmbedder{err: embedErr},
		fakeEmbeddingStore{},
		nil,
		nil,
		fakeEnrichmentStore{},
		nil,
		nil,
	)

	req := search.NewMultiRequest(10, "test query", "test query", nil, search.NewFilters())
	_, err := svc.Search(context.Background(), req)

	if err == nil {
		t.Fatal("expected error when embedding fails, got nil")
	}
	if !errors.Is(err, embedErr) {
		t.Errorf("expected error to wrap %v, got %v", embedErr, err)
	}
}

func TestSearch_KeywordsProduceSeparateFusionLists(t *testing.T) {
	// Two keywords each returning different results. If they're separate
	// fusion lists, a snippet appearing in both gets boosted by RRF.
	// If flattened into one list, the second occurrence gets a worse rank.
	now := time.Now()
	enrichments := []enrichment.Enrichment{
		enrichment.ReconstructEnrichment(1, enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code a", ".go", now, now),
		enrichment.ReconstructEnrichment(2, enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code b", ".go", now, now),
		enrichment.ReconstructEnrichment(3, enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "code c", ".go", now, now),
	}

	bm25 := fakeBM25Store{
		resultsByKeyword: map[string][]search.Result{
			"auth": {
				search.NewResult("1", 5.0),
				search.NewResult("2", 3.0),
			},
			"login": {
				search.NewResult("2", 4.0),
				search.NewResult("3", 2.0),
			},
		},
	}

	svc := NewSearch(nil, nil, nil, bm25, fakeEnrichmentStore{enrichments: enrichments}, nil, nil)

	req := search.NewMultiRequest(10, "", "", []string{"auth", "login"}, search.NewFilters())
	result, err := svc.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scores := result.FusedScores()

	// Snippet "2" appears in both keyword lists, so it should have the highest
	// fused score (sum of RRF scores from two lists)
	if scores["2"] <= scores["1"] {
		t.Errorf("snippet 2 (in both lists) should score higher than snippet 1 (in one list): got 2=%f, 1=%f", scores["2"], scores["1"])
	}
	if scores["2"] <= scores["3"] {
		t.Errorf("snippet 2 (in both lists) should score higher than snippet 3 (in one list): got 2=%f, 3=%f", scores["2"], scores["3"])
	}
}

func TestSearch_TextVectorFailure_ReturnsError(t *testing.T) {
	searchErr := errors.New("vector store down")
	svc := NewSearch(
		fakeEmbedder{vectors: [][]float64{{0.1, 0.2, 0.3}}},
		fakeEmbeddingStore{err: searchErr},
		nil,
		nil,
		fakeEnrichmentStore{},
		nil,
		nil,
	)

	req := search.NewMultiRequest(10, "test", "test", nil, search.NewFilters())
	_, err := svc.Search(context.Background(), req)

	if err == nil {
		t.Fatal("expected error when text vector search fails, got nil")
	}
	if !errors.Is(err, searchErr) {
		t.Errorf("expected error to wrap %v, got %v", searchErr, err)
	}
}

func TestSearch_BM25Failure_ReturnsError(t *testing.T) {
	bm25Err := errors.New("bm25 connection lost")
	svc := NewSearch(
		nil,
		nil,
		nil,
		fakeBM25Store{err: bm25Err},
		fakeEnrichmentStore{},
		nil,
		nil,
	)

	req := search.NewMultiRequest(10, "", "", []string{"test"}, search.NewFilters())
	_, err := svc.Search(context.Background(), req)

	if err == nil {
		t.Fatal("expected error when bm25 search fails, got nil")
	}
	if !errors.Is(err, bm25Err) {
		t.Errorf("expected error to wrap %v, got %v", bm25Err, err)
	}
}

func TestSearch_NoStoresConfigured_ReturnsEmpty(t *testing.T) {
	svc := NewSearch(nil, nil, nil, nil, fakeEnrichmentStore{}, nil, nil)

	req := search.NewMultiRequest(10, "test", "test", []string{"keyword"}, search.NewFilters())
	result, err := svc.Search(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count() != 0 {
		t.Errorf("expected 0 results, got %d", result.Count())
	}
}

func TestOrderByScore(t *testing.T) {
	now := time.Now()
	enrichments := []enrichment.Enrichment{
		enrichment.ReconstructEnrichment(1, enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "low", ".go", now, now),
		enrichment.ReconstructEnrichment(2, enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "high", ".go", now, now),
		enrichment.ReconstructEnrichment(3, enrichment.TypeDevelopment, enrichment.SubtypeSnippet, enrichment.EntityTypeSnippet, "mid", ".go", now, now),
	}

	scores := map[string]float64{
		"1": 0.1,
		"2": 0.9,
		"3": 0.5,
	}

	ordered := orderByScore(enrichments, scores)

	if len(ordered) != 3 {
		t.Fatalf("expected 3 results, got %d", len(ordered))
	}
	if ordered[0].ID() != 2 {
		t.Errorf("expected first result ID=2, got %d", ordered[0].ID())
	}
	if ordered[1].ID() != 3 {
		t.Errorf("expected second result ID=3, got %d", ordered[1].ID())
	}
	if ordered[2].ID() != 1 {
		t.Errorf("expected third result ID=1, got %d", ordered[2].ID())
	}
}
