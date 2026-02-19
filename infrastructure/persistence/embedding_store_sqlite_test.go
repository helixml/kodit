package persistence

import (
	"context"
	"testing"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDB creates an in-memory SQLite database for testing.
// Cannot use testdb package here due to import cycle (testdb imports persistence).
func newTestDB(t *testing.T) database.Database {
	t.Helper()
	ctx := context.Background()
	db, err := database.NewDatabase(ctx, "sqlite:///:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
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

func TestSQLiteEmbeddingStore_SaveAllAndSearch(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, nil)
	require.NoError(t, err)
	ctx := context.Background()

	// Pre-computed embeddings (simulates what EmbeddingService would provide)
	embeddings := []search.Embedding{
		search.NewEmbedding("snippet1", []float64{1.0, 0.5, 0.0, 0.0}),
		search.NewEmbedding("snippet2", []float64{0.0, 1.0, 0.5, 0.0}),
		search.NewEmbedding("snippet3", []float64{0.0, 0.0, 1.0, 0.5}),
	}
	err = store.SaveAll(ctx, embeddings)
	require.NoError(t, err)

	// Search should return results
	results, err := store.Search(ctx, search.WithEmbedding([]float64{1.0, 0.5, 0.0, 0.0}), repository.WithLimit(10))
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

func TestSQLiteEmbeddingStore_SaveAllEmpty(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, nil)
	require.NoError(t, err)
	ctx := context.Background()

	err = store.SaveAll(ctx, []search.Embedding{})
	require.NoError(t, err)
}

func TestSQLiteEmbeddingStore_Search_NoEmbedding(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, nil)
	require.NoError(t, err)
	ctx := context.Background()

	results, err := store.Search(ctx)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSQLiteEmbeddingStore_SaveAllDuplicates(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, nil)
	require.NoError(t, err)
	ctx := context.Background()

	embeddings := []search.Embedding{
		search.NewEmbedding("snippet1", []float64{1.0, 0.0, 0.0, 0.0}),
	}
	err = store.SaveAll(ctx, embeddings)
	require.NoError(t, err)

	// Save the same ID again with a different vector — should upsert, not error
	embeddings2 := []search.Embedding{
		search.NewEmbedding("snippet1", []float64{0.0, 1.0, 0.0, 0.0}),
	}
	err = store.SaveAll(ctx, embeddings2)
	require.NoError(t, err)
}

func TestSQLiteEmbeddingStore_Exists(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, nil)
	require.NoError(t, err)
	ctx := context.Background()

	// Initially does not exist
	has, err := store.Exists(ctx, search.WithSnippetID("snippet1"))
	require.NoError(t, err)
	assert.False(t, has)

	// Save an embedding
	err = store.SaveAll(ctx, []search.Embedding{
		search.NewEmbedding("snippet1", []float64{1.0, 0.0, 0.0, 0.0}),
	})
	require.NoError(t, err)

	// Now should exist
	has, err = store.Exists(ctx, search.WithSnippetID("snippet1"))
	require.NoError(t, err)
	assert.True(t, has)
}

func TestSQLiteEmbeddingStore_DeleteBy(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, nil)
	require.NoError(t, err)
	ctx := context.Background()

	// Save embeddings
	err = store.SaveAll(ctx, []search.Embedding{
		search.NewEmbedding("snippet1", []float64{1.0, 0.0, 0.0, 0.0}),
		search.NewEmbedding("snippet2", []float64{0.0, 1.0, 0.0, 0.0}),
	})
	require.NoError(t, err)

	// Delete one
	err = store.DeleteBy(ctx, search.WithSnippetIDs([]string{"snippet1"}))
	require.NoError(t, err)

	// Verify deletion
	has, err := store.Exists(ctx, search.WithSnippetID("snippet1"))
	require.NoError(t, err)
	assert.False(t, has)

	// Other should still exist
	has, err = store.Exists(ctx, search.WithSnippetID("snippet2"))
	require.NoError(t, err)
	assert.True(t, has)
}

func TestSQLiteEmbeddingStore_SearchWithFilter(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, nil)
	require.NoError(t, err)
	ctx := context.Background()

	// Save embeddings
	err = store.SaveAll(ctx, []search.Embedding{
		search.NewEmbedding("snippet1", []float64{1.0, 0.0, 0.0, 0.0}),
		search.NewEmbedding("snippet2", []float64{0.0, 1.0, 0.0, 0.0}),
		search.NewEmbedding("snippet3", []float64{0.0, 0.0, 1.0, 0.0}),
	})
	require.NoError(t, err)

	// Search with filter
	results, err := store.Search(ctx,
		search.WithEmbedding([]float64{1.0, 0.0, 0.0, 0.0}),
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

func TestSQLiteEmbeddingStore_Find(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLiteEmbeddingStore(db, TaskNameCode, nil)
	require.NoError(t, err)
	ctx := context.Background()

	// Save embeddings
	err = store.SaveAll(ctx, []search.Embedding{
		search.NewEmbedding("snippet1", []float64{1.0, 0.0, 0.0, 0.0}),
		search.NewEmbedding("snippet2", []float64{0.0, 1.0, 0.0, 0.0}),
	})
	require.NoError(t, err)

	// Find matching embeddings
	found, err := store.Find(ctx, search.WithSnippetIDs([]string{"snippet1", "snippet2", "snippet3"}))
	require.NoError(t, err)
	assert.Len(t, found, 2)

	ids := make(map[string]bool, len(found))
	for _, emb := range found {
		ids[emb.SnippetID()] = true
	}
	assert.True(t, ids["snippet1"])
	assert.True(t, ids["snippet2"])
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
