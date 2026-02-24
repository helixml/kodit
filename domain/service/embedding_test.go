package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeEmbedder struct {
	calls [][]string
	errAt int // batch index at which to return an error; -1 = never
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

// --- helpers ---

func testBudget() search.TokenBudget {
	// Large char budget and batch size so existing count-based tests are unaffected.
	b, _ := search.NewTokenBudget(1000000)
	return b.WithMaxBatchSize(1000000)
}

// --- tests ---

func TestEmbeddingService_Index_EmptyRequest(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}
	svc, err := NewEmbedding(store, embedder, testBudget(), 1)
	require.NoError(t, err)

	err = svc.Index(context.Background(), search.NewIndexRequest(nil))
	require.NoError(t, err)
	require.Empty(t, embedder.calls)
	require.Empty(t, store.saved)
}

func TestEmbeddingService_Index_SingleBatch(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}
	svc, err := NewEmbedding(store, embedder, testBudget(), 1)
	require.NoError(t, err)

	documents := make([]search.Document, 5)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), fmt.Sprintf("text %d", i))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	require.Len(t, embedder.calls, 1, "5 short docs fit in one batch")
	require.Len(t, store.saved, 1, "1 SaveAll call")
	require.Len(t, store.saved[0], 5)
}

func TestEmbeddingService_Index_MultipleBatches(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 30-char budget. Each doc "aaaaaaaaaa" is 10 chars, so 3 fit per batch.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	require.Len(t, embedder.calls, 3, "7 docs at 10 chars with 30-char budget = 3 batches")
	require.Len(t, embedder.calls[0], 3)
	require.Len(t, embedder.calls[1], 3)
	require.Len(t, embedder.calls[2], 1)

	require.Len(t, store.saved, 3, "3 SaveAll calls")
	require.Len(t, store.saved[0], 3)
	require.Len(t, store.saved[1], 3)
	require.Len(t, store.saved[2], 1)
}

func TestEmbeddingService_Index_ProgressCallback(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 30-char budget. Each doc "aaaaaaaaaa" is 10 chars, so 3 fit per batch.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
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
		{3, 7},
		{6, 7},
		{7, 7},
	}, calls)
}

func TestEmbeddingService_Index_Deduplication(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{
		existing: map[string]search.Embedding{
			"id-0": search.NewEmbedding("id-0", []float64{1, 2, 3}),
			"id-2": search.NewEmbedding("id-2", []float64{4, 5, 6}),
		},
		saveErr: -1,
	}
	svc, err := NewEmbedding(store, embedder, testBudget(), 1)
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
	embedder := &fakeEmbedder{errAt: 1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 30-char budget, 10-char docs → 3 batches of 3/3/1.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents),
		search.WithMaxFailureRate(0),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "embed batch")
	require.Contains(t, err.Error(), "1 of 3 embedding batches failed")

	// All 3 batches attempted; batch 0 and 2 saved, batch 1 failed embed.
	require.Len(t, embedder.calls, 3, "all batches attempted despite mid-batch error")
	require.Len(t, store.saved, 2, "2 successful saves (batch 0 and 2)")
}

func TestEmbeddingService_Index_SaveErrorMidBatch(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: 1}

	// 30-char budget, 10-char docs → 3 batches of 3/3/1.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents),
		search.WithMaxFailureRate(0),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "save batch")
	require.Contains(t, err.Error(), "1 of 3 embedding batches failed")

	// All 3 batches attempted for embed and save; save at index 1 failed.
	require.Len(t, embedder.calls, 3, "all batches embedded")
	require.Len(t, store.saved, 3, "all 3 save attempts made")
}

func TestEmbeddingService_Index_BatchErrorCallback(t *testing.T) {
	embedder := &fakeEmbedder{errAt: 1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 30-char budget, 10-char docs → 3 batches of 3/3/1.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
	}

	type batchErrCall struct {
		start int
		end   int
		err   string
	}
	var errCalls []batchErrCall

	err = svc.Index(context.Background(), search.NewIndexRequest(documents),
		search.WithBatchError(func(batchStart, batchEnd int, err error) {
			errCalls = append(errCalls, batchErrCall{batchStart, batchEnd, err.Error()})
		}),
	)
	require.Error(t, err)

	require.Len(t, errCalls, 1, "batch error callback called once for the failed batch")
	require.Equal(t, 3, errCalls[0].start)
	require.Equal(t, 6, errCalls[0].end)
	require.Contains(t, errCalls[0].err, "embed error at batch 1")
}

func TestEmbeddingService_Index_InvalidDocumentsFiltered(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}
	svc, err := NewEmbedding(store, embedder, testBudget(), 1)
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

func TestEmbeddingService_Index_TruncatesLargeTexts(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	budget, err := search.NewTokenBudget(20)
	require.NoError(t, err)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := []search.Document{
		search.NewDocument("id-0", "short"),
		search.NewDocument("id-1", strings.Repeat("x", 50)),
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	// "short" (5 chars) fits alone. The 50-char text is truncated to 20.
	// Both exceed 20 together so they split into separate batches.
	require.Len(t, embedder.calls, 2)
	require.Equal(t, "short", embedder.calls[0][0])
	require.Len(t, embedder.calls[1][0], 20, "text truncated to maxChars")
}

func TestEmbeddingService_Index_SplitsByCharBudget(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 30 chars budget. Each doc is 10 chars, so 3 fit per batch.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	// 7 docs * 10 chars = 70 chars. At 30 chars/batch: batches of 3, 3, 1.
	require.Len(t, embedder.calls, 3)
	require.Len(t, embedder.calls[0], 3)
	require.Len(t, embedder.calls[1], 3)
	require.Len(t, embedder.calls[2], 1)
}

func TestEmbeddingService_Index_LargeDocGetsOwnBatch(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 20 chars budget. Large doc exceeds it, gets its own batch.
	budget, err := search.NewTokenBudget(20)
	require.NoError(t, err)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := []search.Document{
		search.NewDocument("id-0", strings.Repeat("a", 5)),
		search.NewDocument("id-1", strings.Repeat("b", 50)), // exceeds batch budget, gets own batch
		search.NewDocument("id-2", strings.Repeat("c", 5)),
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	require.Len(t, embedder.calls, 3)
	require.Len(t, embedder.calls[0], 1, "first small doc alone (next doc would overflow)")
	require.Len(t, embedder.calls[1], 1, "large doc alone in its own batch")
	require.Len(t, embedder.calls[2], 1, "last small doc alone")
}

func TestEmbeddingService_Index_ToleratesPartialFailure(t *testing.T) {
	embedder := &fakeEmbedder{errAt: 1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 30-char budget, 10-char docs → 3 batches of 3/3/1.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
	}

	// 1 of 3 batches fails (~33%), tolerance is 50% → no error.
	err = svc.Index(context.Background(), search.NewIndexRequest(documents),
		search.WithMaxFailureRate(0.5),
	)
	require.NoError(t, err)

	require.Len(t, embedder.calls, 3, "all batches attempted")
	require.Len(t, store.saved, 2, "2 successful saves")
}

func TestEmbeddingService_Index_ExceedsFailureTolerance(t *testing.T) {
	// Fail at batch 0 and batch 1 (2 of 3 batches).
	embedder := &fakeEmbedder{errAt: 0}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 30-char budget, 10-char docs → 3 batches of 3/3/1.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
	}

	// 1 of 3 batches fails (~33%), tolerance is 10% → error.
	err = svc.Index(context.Background(), search.NewIndexRequest(documents),
		search.WithMaxFailureRate(0.1),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "1 of 3 embedding batches failed")
}

func TestEmbeddingService_Index_ParallelBatches(t *testing.T) {
	embedder := &fakeEmbedder{errAt: -1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 30-char budget, 10-char docs → 3 batches of 3/3/1.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	svc, err := NewEmbedding(store, embedder, budget, 3)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
	}

	err = svc.Index(context.Background(), search.NewIndexRequest(documents))
	require.NoError(t, err)

	// All 7 documents embedded and saved across 3 batches.
	require.Len(t, embedder.calls, 3)
	require.Len(t, store.saved, 3)

	// Verify all documents were saved.
	total := 0
	for _, batch := range store.saved {
		total += len(batch)
	}
	require.Equal(t, 7, total)
}

func TestEmbeddingService_Index_ProgressReachesTotalOnPartialFailure(t *testing.T) {
	// Fail batch 1 of 3.
	embedder := &fakeEmbedder{errAt: 1}
	store := &fakeEmbeddingStore{existing: map[string]search.Embedding{}, saveErr: -1}

	// 30-char budget, 10-char docs → 3 batches of 3/3/1.
	budget, err := search.NewTokenBudget(30)
	require.NoError(t, err)
	budget = budget.WithMaxBatchSize(100)

	// Parallelism 1 for deterministic ordering.
	svc, err := NewEmbedding(store, embedder, budget, 1)
	require.NoError(t, err)

	documents := make([]search.Document, 7)
	for i := range documents {
		documents[i] = search.NewDocument(fmt.Sprintf("id-%d", i), strings.Repeat("a", 10))
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
		search.WithMaxFailureRate(0.5),
	)
	require.NoError(t, err)

	// Every batch must produce a progress callback, even failed ones,
	// so the final completed count equals total.
	require.Len(t, calls, 3, "progress called once per batch including the failed one")
	require.Equal(t, 7, calls[len(calls)-1].completed,
		"final progress must report all documents as processed")
	require.Equal(t, 7, calls[len(calls)-1].total)
}
