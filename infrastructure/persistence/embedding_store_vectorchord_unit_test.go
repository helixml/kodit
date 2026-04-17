package persistence

import (
	"context"
	"testing"

	"github.com/helixml/kodit/domain/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectorChordEmbeddingStore_GuardReturnsEmptyWhenNotReady(t *testing.T) {
	ctx := context.Background()

	// Construct directly — tableReady defaults to false, no DB needed
	// since all methods short-circuit before touching the Repository.
	store := &VectorChordEmbeddingStore{}

	t.Run("Find", func(t *testing.T) {
		results, err := store.Find(ctx, search.WithSnippetIDs([]string{"1"}))
		require.NoError(t, err)
		assert.Nil(t, results)
	})

	t.Run("DeleteBy", func(t *testing.T) {
		err := store.DeleteBy(ctx, search.WithSnippetIDs([]string{"1"}))
		assert.NoError(t, err)
	})

	t.Run("Exists", func(t *testing.T) {
		exists, err := store.Exists(ctx, search.WithSnippetIDs([]string{"1"}))
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Search", func(t *testing.T) {
		results, err := store.Search(ctx)
		require.NoError(t, err)
		assert.Nil(t, results)
	})
}

func TestVectorChordEmbeddingStore_TableReadyFlag(t *testing.T) {
	store := &VectorChordEmbeddingStore{}

	assert.False(t, store.tableReady.Load(), "should start as false")

	store.tableReady.Store(true)
	assert.True(t, store.tableReady.Load(), "should be true after Store(true)")
}
