package service

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// dupEmbeddingStore is a fake EmbeddingStore for duplicate detection tests.
type dupEmbeddingStore struct {
	embeddings []search.Embedding
	err        error
}

func (d *dupEmbeddingStore) SaveAll(_ context.Context, _ []search.Embedding) error { return nil }
func (d *dupEmbeddingStore) Find(_ context.Context, _ ...repository.Option) ([]search.Embedding, error) {
	return nil, nil
}
func (d *dupEmbeddingStore) Search(_ context.Context, _ ...repository.Option) ([]search.Result, error) {
	return nil, nil
}
func (d *dupEmbeddingStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return false, nil
}
func (d *dupEmbeddingStore) FindAll(_ context.Context, _ search.Filters) ([]search.Embedding, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.embeddings, nil
}
func (d *dupEmbeddingStore) DeleteBy(_ context.Context, _ ...repository.Option) error { return nil }

func newDupService(embeddings []search.Embedding) *DuplicateSearch {
	store := &dupEmbeddingStore{embeddings: embeddings}
	return NewDuplicateSearch(store, zerolog.Nop())
}

func TestFindDuplicates_NoStore_ReturnsEmpty(t *testing.T) {
	svc := NewDuplicateSearch(nil, zerolog.Nop())
	pairs, truncated, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.90, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if truncated {
		t.Error("want truncated=false, got true")
	}
	if len(pairs) != 0 {
		t.Errorf("want 0 pairs, got %d", len(pairs))
	}
}

func TestFindDuplicates_EmptyEmbeddings_ReturnsEmpty(t *testing.T) {
	svc := newDupService(nil)
	pairs, truncated, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.90, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if truncated {
		t.Error("want truncated=false, got true")
	}
	if len(pairs) != 0 {
		t.Errorf("want 0 pairs, got %d", len(pairs))
	}
}

func TestFindDuplicates_SingleEmbedding_ReturnsEmpty(t *testing.T) {
	embeddings := []search.Embedding{
		search.NewEmbedding("1", []float64{1, 0, 0}),
	}
	svc := newDupService(embeddings)
	pairs, _, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.90, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("want 0 pairs, got %d", len(pairs))
	}
}

func TestFindDuplicates_IdenticalVectors_ReturnsSimilarityOne(t *testing.T) {
	vec := []float64{1.0, 0.5, 0.3}
	embeddings := []search.Embedding{
		search.NewEmbedding("1", vec),
		search.NewEmbedding("2", vec),
	}
	svc := newDupService(embeddings)
	pairs, _, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.90, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("want 1 pair, got %d", len(pairs))
	}
	if pairs[0].Similarity < 0.999 {
		t.Errorf("want similarity ~1.0, got %f", pairs[0].Similarity)
	}
	// Verify snippet IDs
	gotA, gotB := pairs[0].SnippetIDA, pairs[0].SnippetIDB
	if !((gotA == "1" && gotB == "2") || (gotA == "2" && gotB == "1")) {
		t.Errorf("unexpected snippet IDs: A=%s B=%s", gotA, gotB)
	}
}

func TestFindDuplicates_DissimilarVectors_NotReturned(t *testing.T) {
	embeddings := []search.Embedding{
		search.NewEmbedding("1", []float64{1, 0, 0}),
		search.NewEmbedding("2", []float64{0, 1, 0}), // orthogonal: cosine similarity = 0
	}
	svc := newDupService(embeddings)
	pairs, _, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.90, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("want 0 pairs, got %d", len(pairs))
	}
}

func TestFindDuplicates_ThresholdBoundary_Inclusive(t *testing.T) {
	// Two vectors with known cosine similarity
	// cos([1,1,0], [1,0,0]) = 1/sqrt(2) ≈ 0.7071
	embeddings := []search.Embedding{
		search.NewEmbedding("1", []float64{1, 1, 0}),
		search.NewEmbedding("2", []float64{1, 0, 0}),
	}
	svc := newDupService(embeddings)

	// Threshold just below the similarity → should be returned
	pairs, _, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.70, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 1 {
		t.Errorf("threshold below sim: want 1 pair, got %d", len(pairs))
	}

	// Threshold above the similarity → should not be returned
	pairs, _, err = svc.FindDuplicates(context.Background(), []int64{1}, 0.80, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("threshold above sim: want 0 pairs, got %d", len(pairs))
	}
}

func TestFindDuplicates_LimitCap(t *testing.T) {
	// 4 embeddings → up to 6 pairs; we set limit=3
	vec := []float64{1, 0, 0}
	embeddings := []search.Embedding{
		search.NewEmbedding("1", vec),
		search.NewEmbedding("2", vec),
		search.NewEmbedding("3", vec),
		search.NewEmbedding("4", vec),
	}
	svc := newDupService(embeddings)
	pairs, _, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.90, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 3 {
		t.Errorf("want 3 pairs (capped), got %d", len(pairs))
	}
}

func TestFindDuplicates_SortedBySimilarityDescending(t *testing.T) {
	// 3 embeddings: pair (1,2) is very similar, pair (1,3) is less similar
	embeddings := []search.Embedding{
		search.NewEmbedding("1", []float64{1.0, 0.0}),
		search.NewEmbedding("2", []float64{0.99, 0.01}), // close to "1"
		search.NewEmbedding("3", []float64{0.5, 0.5}),   // farther from "1"
	}
	svc := newDupService(embeddings)
	pairs, _, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.0, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) < 2 {
		t.Fatalf("want at least 2 pairs, got %d", len(pairs))
	}
	for i := 1; i < len(pairs); i++ {
		if pairs[i].Similarity > pairs[i-1].Similarity {
			t.Errorf("pairs not sorted: pairs[%d].Similarity=%f > pairs[%d].Similarity=%f",
				i, pairs[i].Similarity, i-1, pairs[i-1].Similarity)
		}
	}
}

func TestFindDuplicates_ZeroVector_Skipped(t *testing.T) {
	// Zero vectors should not produce NaN similarities and should be skipped
	embeddings := []search.Embedding{
		search.NewEmbedding("1", []float64{0, 0, 0}),
		search.NewEmbedding("2", []float64{1, 0, 0}),
	}
	svc := newDupService(embeddings)
	pairs, _, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.0, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Zero-magnitude vector should be skipped, no pairs returned
	if len(pairs) != 0 {
		t.Errorf("want 0 pairs (zero vector skipped), got %d", len(pairs))
	}
}

func TestFindDuplicates_Truncated_WhenExceedsMax(t *testing.T) {
	// Create more embeddings than maxEmbeddingsForDuplicate
	n := maxEmbeddingsForDuplicate + 1
	embeddings := make([]search.Embedding, n)
	for i := range embeddings {
		embeddings[i] = search.NewEmbedding(
			string(rune('A'+i%26))+string(rune('a'+i%26)),
			[]float64{float64(i), 1, 0},
		)
	}
	svc := newDupService(embeddings)
	_, truncated, err := svc.FindDuplicates(context.Background(), []int64{1}, 0.90, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !truncated {
		t.Error("want truncated=true when N > maxEmbeddingsForDuplicate")
	}
}
