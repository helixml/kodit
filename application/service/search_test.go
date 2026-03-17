package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// fakeEmbedder implements search.Embedder for testing.
// Genuine fake: the real embedder calls an external model API.
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
// Genuine fake: the real store requires pgvector for similarity search.
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
// Genuine fake: the real store requires ParadeDB for BM25 ranking.
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

// seedEnrichments creates enrichments in the real store and returns them in insertion order.
func seedEnrichments(t *testing.T, stores testStores, contents []string) []enrichment.Enrichment {
	t.Helper()
	ctx := context.Background()
	result := make([]enrichment.Enrichment, len(contents))
	for i, content := range contents {
		e := enrichment.NewEnrichment(
			enrichment.TypeDevelopment,
			enrichment.SubtypeSnippet,
			enrichment.EntityTypeSnippet,
			content,
		)
		saved, err := stores.enrichments.Save(ctx, e)
		if err != nil {
			t.Fatalf("save enrichment %d: %v", i, err)
		}
		result[i] = saved
	}
	return result
}

func TestSearch_EmbeddingFailure_ReturnsError(t *testing.T) {
	stores := newTestStores(t)
	embedErr := errors.New("model unavailable")
	svc := NewSearch(
		fakeEmbedder{err: embedErr},
		fakeEmbeddingStore{},
		nil,
		nil,
		stores.enrichments,
		nil,
		zerolog.Nop(),
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
	stores := newTestStores(t)
	enrichments := seedEnrichments(t, stores, []string{"code a", "code b", "code c"})

	id1 := strconv.FormatInt(enrichments[0].ID(), 10)
	id2 := strconv.FormatInt(enrichments[1].ID(), 10)
	id3 := strconv.FormatInt(enrichments[2].ID(), 10)

	bm25 := fakeBM25Store{
		resultsByKeyword: map[string][]search.Result{
			"auth": {
				search.NewResult(id1, 5.0),
				search.NewResult(id2, 3.0),
			},
			"login": {
				search.NewResult(id2, 4.0),
				search.NewResult(id3, 2.0),
			},
		},
	}

	svc := NewSearch(nil, nil, nil, bm25, stores.enrichments, nil, zerolog.Nop())

	req := search.NewMultiRequest(10, "", "", []string{"auth", "login"}, search.NewFilters())
	result, err := svc.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scores := result.FusedScores()

	// Enrichment id2 appears in both keyword lists, so it should have the highest
	// fused score (sum of RRF scores from two lists).
	if scores[id2] <= scores[id1] {
		t.Errorf("enrichment %s (in both lists) should score higher than %s (in one list): got %s=%f, %s=%f", id2, id1, id2, scores[id2], id1, scores[id1])
	}
	if scores[id2] <= scores[id3] {
		t.Errorf("enrichment %s (in both lists) should score higher than %s (in one list): got %s=%f, %s=%f", id2, id3, id2, scores[id2], id3, scores[id3])
	}
}

func TestSearch_TextVectorFailure_ReturnsError(t *testing.T) {
	stores := newTestStores(t)
	searchErr := errors.New("vector store down")
	svc := NewSearch(
		fakeEmbedder{vectors: [][]float64{{0.1, 0.2, 0.3}}},
		fakeEmbeddingStore{err: searchErr},
		nil,
		nil,
		stores.enrichments,
		nil,
		zerolog.Nop(),
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
	stores := newTestStores(t)
	bm25Err := errors.New("bm25 connection lost")
	svc := NewSearch(
		nil,
		nil,
		nil,
		fakeBM25Store{err: bm25Err},
		stores.enrichments,
		nil,
		zerolog.Nop(),
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
	stores := newTestStores(t)
	svc := NewSearch(nil, nil, nil, nil, stores.enrichments, nil, zerolog.Nop())

	req := search.NewMultiRequest(10, "test", "test", []string{"keyword"}, search.NewFilters())
	result, err := svc.Search(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count() != 0 {
		t.Errorf("expected 0 results, got %d", result.Count())
	}
}

func TestSearchCodeWithScores_OrdersByScoreDescending(t *testing.T) {
	stores := newTestStores(t)
	enrichments := seedEnrichments(t, stores, []string{"low", "high", "mid"})

	id1 := strconv.FormatInt(enrichments[0].ID(), 10)
	id2 := strconv.FormatInt(enrichments[1].ID(), 10)
	id3 := strconv.FormatInt(enrichments[2].ID(), 10)

	codeVectorStore := fakeEmbeddingStore{
		results: []search.Result{
			search.NewResult(id2, 0.9),
			search.NewResult(id3, 0.5),
			search.NewResult(id1, 0.1),
		},
	}

	svc := NewSearch(
		fakeEmbedder{vectors: [][]float64{{0.1, 0.2}}},
		nil,
		codeVectorStore,
		nil,
		stores.enrichments,
		nil,
		zerolog.Nop(),
	)

	results, scores, err := svc.SearchCodeWithScores(context.Background(), "test", 10, search.NewFilters())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Highest score first.
	if results[0].ID() != enrichments[1].ID() {
		t.Errorf("expected first result ID=%d (score 0.9), got ID=%d (score %f)", enrichments[1].ID(), results[0].ID(), scores[strconv.FormatInt(results[0].ID(), 10)])
	}
	if results[1].ID() != enrichments[2].ID() {
		t.Errorf("expected second result ID=%d (score 0.5), got ID=%d (score %f)", enrichments[2].ID(), results[1].ID(), scores[strconv.FormatInt(results[1].ID(), 10)])
	}
	if results[2].ID() != enrichments[0].ID() {
		t.Errorf("expected third result ID=%d (score 0.1), got ID=%d (score %f)", enrichments[0].ID(), results[2].ID(), scores[strconv.FormatInt(results[2].ID(), 10)])
	}
}

func TestSearchKeywordsWithScores_ReturnsResults(t *testing.T) {
	stores := newTestStores(t)
	enrichments := seedEnrichments(t, stores, []string{"auth code", "login code"})

	id1 := strconv.FormatInt(enrichments[0].ID(), 10)
	id2 := strconv.FormatInt(enrichments[1].ID(), 10)

	bm25 := fakeBM25Store{
		resultsByKeyword: map[string][]search.Result{
			"auth": {
				search.NewResult(id2, 8.0),
				search.NewResult(id1, 3.0),
			},
		},
	}

	svc := NewSearch(nil, nil, nil, bm25, stores.enrichments, nil, zerolog.Nop())

	results, scores, err := svc.SearchKeywordsWithScores(context.Background(), "auth", 10, search.NewFilters())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Results should be ordered by score descending.
	if results[0].ID() != enrichments[1].ID() {
		t.Errorf("expected first result ID=%d (score 8.0), got ID=%d", enrichments[1].ID(), results[0].ID())
	}
	if results[1].ID() != enrichments[0].ID() {
		t.Errorf("expected second result ID=%d (score 3.0), got ID=%d", enrichments[0].ID(), results[1].ID())
	}

	if scores[id2] != 8.0 {
		t.Errorf("expected score 8.0 for ID %s, got %f", id2, scores[id2])
	}
	if scores[id1] != 3.0 {
		t.Errorf("expected score 3.0 for ID %s, got %f", id1, scores[id1])
	}
}

func TestSearchKeywordsWithScores_NilStore(t *testing.T) {
	stores := newTestStores(t)
	svc := NewSearch(nil, nil, nil, nil, stores.enrichments, nil, zerolog.Nop())

	results, scores, err := svc.SearchKeywordsWithScores(context.Background(), "auth", 10, search.NewFilters())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %d", len(results))
	}
	if scores != nil {
		t.Errorf("expected nil scores, got %v", scores)
	}
}

func TestSearchKeywordsWithScores_NoResults(t *testing.T) {
	stores := newTestStores(t)
	bm25 := fakeBM25Store{
		resultsByKeyword: map[string][]search.Result{},
	}

	svc := NewSearch(nil, nil, nil, bm25, stores.enrichments, nil, zerolog.Nop())

	results, scores, err := svc.SearchKeywordsWithScores(context.Background(), "nonexistent", 10, search.NewFilters())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %d", len(results))
	}
	if scores != nil {
		t.Errorf("expected nil scores, got %v", scores)
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
