package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeEmbedder struct {
	calls    [][]string
	capacity int
	errAt    int // batch index at which to return an error; -1 = never
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	idx := len(f.calls)
	f.calls = append(f.calls, texts)
	if f.errAt >= 0 && idx == f.errAt {
		return nil, fmt.Errorf("embed error at batch %d", idx)
	}
	vectors := make([][]float64, len(texts))
	for i := range texts {
		vectors[i] = []float64{0.1, 0.2, 0.3}
	}
	return vectors, nil
}

func (f *fakeEmbedder) Capacity() int { return f.capacity }

type fakeEmbeddingStore struct {
	saved    [][]search.Embedding
	existing map[string]search.Embedding
	saveErr  int // SaveAll call index at which to return an error; -1 = never
}

func (f *fakeEmbeddingStore) SaveAll(_ context.Context, embeddings []search.Embedding) error {
	idx := len(f.saved)
	f.saved = append(f.saved, embeddings)
	if f.saveErr >= 0 && idx == f.saveErr {
		return fmt.Errorf("save error at call %d", idx)
	}
	for _, e := range embeddings {
		f.existing[e.SnippetID()] = e
	}
	return nil
}

func (f *fakeEmbeddingStore) Find(_ context.Context, options ...repository.Option) ([]search.Embedding, error) {
	q := repository.Build(options...)
	ids := search.SnippetIDsFrom(q)
	var result []search.Embedding
	for _, id := range ids {
		if e, ok := f.existing[id]; ok {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeEmbeddingStore) Search(_ context.Context, _ ...repository.Option) ([]search.Result, error) {
	return nil, nil
}

func (f *fakeEmbeddingStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return len(f.existing) > 0, nil
}

func (f *fakeEmbeddingStore) DeleteBy(_ context.Context, _ ...repository.Option) error {
	return nil
}

// --- tests ---

func TestEmbeddingService_Index_EmptyRequest(t *testing.T) {
	embedder := &fakeEmbedder{capacity: 10, errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}
	svc, err := NewEmbedding(store, embedder)
	require.NoError(t, err)

	err = svc.Index(context.Background(), search.NewIndexRequest(nil))
	require.NoError(t, err)
	require.Empty(t, embedder.calls)
	require.Empty(t, store.saved)
}

func TestEmbeddingService_Index_SingleBatch(t *testing.T) {
	embedder := &fakeEmbedder{capacity: 10, errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}
	svc, err := NewEmbedding(store, embedder)
	require.NoError(t, err)

	documents := make([]search.Document, 5)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), fmt.Sprintf("text %d", i))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	require.Len(t, embedder.calls, 1, "5 docs with capacity 10 = 1 Embed call")
	require.Len(t, store.saved, 1, "1 SaveAll call")
	require.Len(t, store.saved[0], 5)
}

func TestEmbeddingService_Index_MultipleBatches(t *testing.T) {
	embedder := &fakeEmbedder{capacity: 10, errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}
	svc, err := NewEmbedding(store, embedder)
	require.NoError(t, err)

	documents := make([]search.Document, 25)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), fmt.Sprintf("text %d", i))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	require.Len(t, embedder.calls, 3, "25 docs with capacity 10 = 3 Embed calls")
	require.Len(t, embedder.calls[0], 10)
	require.Len(t, embedder.calls[1], 10)
	require.Len(t, embedder.calls[2], 5)

	require.Len(t, store.saved, 3, "3 SaveAll calls")
	require.Len(t, store.saved[0], 10)
	require.Len(t, store.saved[1], 10)
	require.Len(t, store.saved[2], 5)
}

func TestEmbeddingService_Index_ProgressCallback(t *testing.T) {
	embedder := &fakeEmbedder{capacity: 10, errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}
	svc, err := NewEmbedding(store, embedder)
	require.NoError(t, err)

	documents := make([]search.Document, 25)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), fmt.Sprintf("text %d", i))
	}

	type call struct {
		completed int
		total     int
	}
	var calls []call

	err = svc.Index(context.Background(), search.NewIndexRequest(documents),
		search.WithProgress(func(completed, total int) {
			calls = append(calls, call{completed, total})
		}),
	)
	require.NoError(t, err)

	require.Equal(t, []call{
		{10, 25},
		{20, 25},
		{25, 25},
	}, calls)
}

func TestEmbeddingService_Index_Deduplication(t *testing.T) {
	embedder := &fakeEmbedder{capacity: 10, errAt: -1}
	store := &fakeEmbeddingStore{
		existing: map[string]search.Embedding{
			"id-0": search.NewEmbedding("id-0", []float64{1, 2, 3}),
			"id-2": search.NewEmbedding("id-2", []float64{4, 5, 6}),
		},
		saveErr: -1,
	}
	svc, err := NewEmbedding(store, embedder)
	require.NoError(t, err)

	documents := []search.Document{
		search.NewDocument("id-0", "already exists"),
		search.NewDocument("id-1", "new doc"),
		search.NewDocument("id-2", "already exists"),
		search.NewDocument("id-3", "new doc"),
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	require.Len(t, embedder.calls, 1)
	require.Len(t, embedder.calls[0], 2, "only 2 new documents embedded")
}

func TestEmbeddingService_Index_EmbedErrorMidBatch(t *testing.T) {
	embedder := &fakeEmbedder{capacity: 10, errAt: 1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}
	svc, err := NewEmbedding(store, embedder)
	require.NoError(t, err)

	documents := make([]search.Document, 25)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), fmt.Sprintf("text %d", i))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.Error(t, err)
	require.Contains(t, err.Error(), "embed batch")

	// Batch 1 was saved before the error.
	require.Len(t, store.saved, 1)
	require.Len(t, store.saved[0], 10)
}

func TestEmbeddingService_Index_SaveErrorMidBatch(t *testing.T) {
	embedder := &fakeEmbedder{capacity: 10, errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: 1}
	svc, err := NewEmbedding(store, embedder)
	require.NoError(t, err)

	documents := make([]search.Document, 25)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), fmt.Sprintf("text %d", i))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.Error(t, err)
	require.Contains(t, err.Error(), "save batch")

	// Batch 0 saved, batch 1 failed.
	require.Len(t, store.saved, 2)
	require.Len(t, store.saved[0], 10)
}

func TestEmbeddingService_Index_InvalidDocumentsFiltered(t *testing.T) {
	embedder := &fakeEmbedder{capacity: 10, errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}
	svc, err := NewEmbedding(store, embedder)
	require.NoError(t, err)

	documents := []search.Document{
		search.NewDocument("", "empty id"),
		search.NewDocument("id-1", "   "),
		search.NewDocument("id-2", "valid text"),
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	require.Len(t, embedder.calls, 1)
	require.Len(t, embedder.calls[0], 1, "only 1 valid document")
}
