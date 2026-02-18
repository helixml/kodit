package search

import (
	"context"
	"testing"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEmbedder implements provider.Embedder for testing.
type fakeEmbedder struct {
	dimension int
}

func newFakeEmbedder(dimension int) *fakeEmbedder {
	return &fakeEmbedder{dimension: dimension}
}

func (f *fakeEmbedder) Embed(_ context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	texts := req.Texts()
	embeddings := make([][]float64, len(texts))
	for i := range texts {
		embedding := make([]float64, f.dimension)
		// Create deterministic embeddings based on text content
		for j := range embedding {
			embedding[j] = float64((i+1)*(j+1)) / float64(f.dimension)
		}
		embeddings[i] = embedding
	}
	return provider.NewEmbeddingResponse(embeddings, provider.NewUsage(len(texts), 0, len(texts))), nil
}

// embedText is a test helper that generates an embedding for a given text.
func embedText(ctx context.Context, embedder *fakeEmbedder, text string) []float64 {
	resp, _ := embedder.Embed(ctx, provider.NewEmbeddingRequest([]string{text}))
	return resp.Embeddings()[0]
}

func testDB(t *testing.T) database.Database {
	t.Helper()
	return testdb.NewPlain(t)
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "opposite vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "zero vector a",
			a:        []float64{0, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 0.0,
		},
		{
			name:     "zero vector b",
			a:        []float64{1, 0, 0},
			b:        []float64{0, 0, 0},
			expected: 0.0,
		},
		{
			name:     "both zero vectors",
			a:        []float64{0, 0, 0},
			b:        []float64{0, 0, 0},
			expected: 0.0,
		},
		{
			name:     "empty vectors",
			a:        []float64{},
			b:        []float64{},
			expected: 0.0,
		},
		{
			name:     "mismatched lengths",
			a:        []float64{1, 0},
			b:        []float64{1, 0, 0},
			expected: 0.0,
		},
		{
			name:     "similar vectors",
			a:        []float64{1, 1, 0},
			b:        []float64{1, 0.9, 0.1},
			expected: 0.9959, // approximately
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestTopKSimilar(t *testing.T) {
	query := []float64{1, 0, 0}
	vectors := []StoredVector{
		NewStoredVector("exact", []float64{1, 0, 0}),
		NewStoredVector("similar", []float64{0.9, 0.1, 0}),
		NewStoredVector("orthogonal", []float64{0, 1, 0}),
		NewStoredVector("opposite", []float64{-1, 0, 0}),
	}

	t.Run("top 2", func(t *testing.T) {
		results := TopKSimilar(query, vectors, 2)
		require.Len(t, results, 2)
		assert.Equal(t, "exact", results[0].SnippetID())
		assert.InDelta(t, 1.0, results[0].Similarity(), 0.001)
		assert.Equal(t, "similar", results[1].SnippetID())
	})

	t.Run("top k larger than results", func(t *testing.T) {
		results := TopKSimilar(query, vectors, 10)
		require.Len(t, results, 4)
	})

	t.Run("k is zero", func(t *testing.T) {
		results := TopKSimilar(query, vectors, 0)
		assert.Empty(t, results)
	})

	t.Run("empty vectors", func(t *testing.T) {
		results := TopKSimilar(query, []StoredVector{}, 5)
		assert.Empty(t, results)
	})
}

func TestTopKSimilarFiltered(t *testing.T) {
	query := []float64{1, 0, 0}
	vectors := []StoredVector{
		NewStoredVector("exact", []float64{1, 0, 0}),
		NewStoredVector("similar", []float64{0.9, 0.1, 0}),
		NewStoredVector("orthogonal", []float64{0, 1, 0}),
	}

	t.Run("filter to subset", func(t *testing.T) {
		allowedIDs := map[string]struct{}{"similar": {}, "orthogonal": {}}
		results := TopKSimilarFiltered(query, vectors, 5, allowedIDs)
		require.Len(t, results, 2)
		assert.Equal(t, "similar", results[0].SnippetID())
		assert.Equal(t, "orthogonal", results[1].SnippetID())
	})

	t.Run("empty filter returns all", func(t *testing.T) {
		results := TopKSimilarFiltered(query, vectors, 5, map[string]struct{}{})
		require.Len(t, results, 3)
	})
}

func TestNewSQLiteVectorStore(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)

	assert.NotNil(t, store)
	assert.NotNil(t, store.logger)
	assert.False(t, store.initialized)
	assert.Equal(t, "kodit_code_embeddings", store.repo.Table())
}

func TestNewSQLiteVectorStore_TextTask(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	store := NewSQLiteVectorStore(db, TaskNameText, embedder, nil)

	assert.Equal(t, "kodit_text_embeddings", store.repo.Table())
}

func TestSQLiteVectorStore_Index_EmptyDocuments(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	err := store.Index(ctx, search.NewIndexRequest([]search.Document{}))
	require.NoError(t, err)
}

func TestSQLiteVectorStore_Find_NoEmbedding(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	// Find with no embedding option returns empty
	results, err := store.Find(ctx)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSQLiteVectorStore_DeleteBy_EmptyIDs(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	err := store.DeleteBy(ctx, search.WithSnippetIDs([]string{}))
	require.NoError(t, err)
}

func TestSQLiteVectorStore_IndexAndFind(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(4)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	// Index some documents
	docs := []search.Document{
		search.NewDocument("snippet1", "function add numbers"),
		search.NewDocument("snippet2", "parse json data"),
		search.NewDocument("snippet3", "calculate sum total"),
	}
	err := store.Index(ctx, search.NewIndexRequest(docs))
	require.NoError(t, err)

	// Find should return results
	emb := embedText(ctx, embedder, "add numbers")
	results, err := store.Find(ctx, search.WithEmbedding(emb), repository.WithLimit(10))
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	// All documents should be in results
	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.SnippetID()] = true
	}
	assert.True(t, ids["snippet1"])
	assert.True(t, ids["snippet2"])
	assert.True(t, ids["snippet3"])
}

func TestSQLiteVectorStore_IndexDuplicates(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(4)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	// Index a document
	docs := []search.Document{
		search.NewDocument("snippet1", "test content"),
	}
	err := store.Index(ctx, search.NewIndexRequest(docs))
	require.NoError(t, err)

	// Index the same document again - should not error
	err = store.Index(ctx, search.NewIndexRequest(docs))
	require.NoError(t, err)
}

func TestSQLiteVectorStore_Exists(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(4)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	// Initially does not exist
	has, err := store.Exists(ctx, search.WithSnippetID("snippet1"))
	require.NoError(t, err)
	assert.False(t, has)

	// Index a document
	docs := []search.Document{
		search.NewDocument("snippet1", "test content"),
	}
	err = store.Index(ctx, search.NewIndexRequest(docs))
	require.NoError(t, err)

	// Now should exist
	has, err = store.Exists(ctx, search.WithSnippetID("snippet1"))
	require.NoError(t, err)
	assert.True(t, has)
}

func TestSQLiteVectorStore_DeleteBy(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(4)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	// Index documents
	docs := []search.Document{
		search.NewDocument("snippet1", "test content one"),
		search.NewDocument("snippet2", "test content two"),
	}
	err := store.Index(ctx, search.NewIndexRequest(docs))
	require.NoError(t, err)

	// Delete one document
	err = store.DeleteBy(ctx, search.WithSnippetIDs([]string{"snippet1"}))
	require.NoError(t, err)

	// Verify deletion
	has, err := store.Exists(ctx, search.WithSnippetID("snippet1"))
	require.NoError(t, err)
	assert.False(t, has)

	// Other document should still exist
	has, err = store.Exists(ctx, search.WithSnippetID("snippet2"))
	require.NoError(t, err)
	assert.True(t, has)
}

func TestSQLiteVectorStore_FindWithFilter(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(4)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	// Index documents
	docs := []search.Document{
		search.NewDocument("snippet1", "test content one"),
		search.NewDocument("snippet2", "test content two"),
		search.NewDocument("snippet3", "test content three"),
	}
	err := store.Index(ctx, search.NewIndexRequest(docs))
	require.NoError(t, err)

	// Find with filter
	emb := embedText(ctx, embedder, "test")
	results, err := store.Find(ctx,
		search.WithEmbedding(emb),
		search.WithSnippetIDs([]string{"snippet1", "snippet3"}),
		repository.WithLimit(10),
	)
	require.NoError(t, err)

	// Should only return filtered snippets
	assert.Len(t, results, 2)
	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.SnippetID()] = true
	}
	assert.True(t, ids["snippet1"])
	assert.True(t, ids["snippet3"])
	assert.False(t, ids["snippet2"])
}

func TestFloat64Slice_ScanValue(t *testing.T) {
	t.Run("scan from bytes", func(t *testing.T) {
		var f Float64Slice
		err := f.Scan([]byte("[1.0, 2.0, 3.0]"))
		require.NoError(t, err)
		assert.Equal(t, Float64Slice{1.0, 2.0, 3.0}, f)
	})

	t.Run("scan from string", func(t *testing.T) {
		var f Float64Slice
		err := f.Scan("[4.0, 5.0]")
		require.NoError(t, err)
		assert.Equal(t, Float64Slice{4.0, 5.0}, f)
	})

	t.Run("scan nil", func(t *testing.T) {
		var f Float64Slice
		err := f.Scan(nil)
		require.NoError(t, err)
		assert.Nil(t, f)
	})

	t.Run("value round trip", func(t *testing.T) {
		original := Float64Slice{1.5, 2.5, 3.5}
		val, err := original.Value()
		require.NoError(t, err)

		var restored Float64Slice
		err = restored.Scan(val)
		require.NoError(t, err)
		assert.Equal(t, original, restored)
	})
}
