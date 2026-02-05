package search

import (
	"context"
	"testing"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	return db
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
	assert.Equal(t, "kodit_code_embeddings", store.tableName)
}

func TestNewSQLiteVectorStore_TextTask(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	store := NewSQLiteVectorStore(db, TaskNameText, embedder, nil)

	assert.Equal(t, "kodit_text_embeddings", store.tableName)
}

func TestSQLiteVectorStore_Index_EmptyDocuments(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	err := store.Index(ctx, search.NewIndexRequest([]search.Document{}))
	require.NoError(t, err)
}

func TestSQLiteVectorStore_Search_EmptyQuery(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	results, err := store.Search(ctx, search.NewRequest("", 10, nil))
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSQLiteVectorStore_Delete_EmptyIDs(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	err := store.Delete(ctx, search.NewDeleteRequest([]string{}))
	require.NoError(t, err)
}

func TestSQLiteVectorStore_IndexAndSearch(t *testing.T) {
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

	// Search should return results
	results, err := store.Search(ctx, search.NewRequest("add numbers", 10, nil))
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

func TestSQLiteVectorStore_HasEmbedding(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(4)
	store := NewSQLiteVectorStore(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	// Initially no embedding
	has, err := store.HasEmbedding(ctx, "snippet1", snippet.EmbeddingTypeCode)
	require.NoError(t, err)
	assert.False(t, has)

	// Index a document
	docs := []search.Document{
		search.NewDocument("snippet1", "test content"),
	}
	err = store.Index(ctx, search.NewIndexRequest(docs))
	require.NoError(t, err)

	// Now should have embedding
	has, err = store.HasEmbedding(ctx, "snippet1", snippet.EmbeddingTypeCode)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestSQLiteVectorStore_EmbeddingsForSnippets(t *testing.T) {
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

	// Retrieve embeddings
	infos, err := store.EmbeddingsForSnippets(ctx, []string{"snippet1", "snippet2"})
	require.NoError(t, err)
	assert.Len(t, infos, 2)

	// Verify embeddings have correct IDs and type
	ids := make(map[string]bool)
	for _, info := range infos {
		ids[info.SnippetID()] = true
		assert.Equal(t, snippet.EmbeddingTypeCode, info.EmbeddingType())
		assert.Len(t, info.Embedding(), 4)
	}
	assert.True(t, ids["snippet1"])
	assert.True(t, ids["snippet2"])
}

func TestSQLiteVectorStore_Delete(t *testing.T) {
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
	err = store.Delete(ctx, search.NewDeleteRequest([]string{"snippet1"}))
	require.NoError(t, err)

	// Verify deletion
	has, err := store.HasEmbedding(ctx, "snippet1", snippet.EmbeddingTypeCode)
	require.NoError(t, err)
	assert.False(t, has)

	// Other document should still exist
	has, err = store.HasEmbedding(ctx, "snippet2", snippet.EmbeddingTypeCode)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestSQLiteVectorStore_SearchWithFilter(t *testing.T) {
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

	// Search with filter
	results, err := store.Search(ctx, search.NewRequest("test", 10, []string{"snippet1", "snippet3"}))
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
