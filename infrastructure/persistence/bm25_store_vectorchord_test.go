package persistence

import (
	"context"
	"os"
	"testing"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVectorChordBM25Store_Integration exercises the full BM25 lifecycle
// (extension creation, tokenizer setup, indexing, search, delete) against a
// real VectorChord instance. The search_path is set in the connection string
// — not via SET — so every pooled connection resolves bm25_catalog and
// tokenizer_catalog functions correctly.
//
// Skipped when VECTORCHORD_TEST_URL is not set.
//
//	VECTORCHORD_TEST_URL="postgresql://postgres:mysecretpassword@localhost:5434/kodit" go test -v -run TestVectorChordBM25Store_Integration ./infrastructure/persistence/
func TestVectorChordBM25Store_Integration(t *testing.T) {
	dsn := os.Getenv("VECTORCHORD_TEST_URL")
	if dsn == "" {
		t.Skip("VECTORCHORD_TEST_URL not set — start vectorchord-clean via docker compose")
	}

	ctx := context.Background()

	db, err := database.NewDatabase(ctx, dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	store, err := NewVectorChordBM25Store(db, nil)
	require.NoError(t, err)

	// Index documents.
	docs := []search.Document{
		search.NewDocument("vc-bm25-1", "kubernetes deployment controller reconciles pods"),
		search.NewDocument("vc-bm25-2", "http router handles incoming web requests"),
		search.NewDocument("vc-bm25-3", "database migration runs schema changes automatically"),
	}
	err = store.Index(ctx, search.NewIndexRequest(docs))
	require.NoError(t, err)

	// Find should return relevant documents.
	results, err := store.Find(ctx,
		search.WithQuery("kubernetes pods"),
		repository.WithLimit(10),
	)
	require.NoError(t, err)
	require.NotEmpty(t, results, "expected BM25 search to return results")

	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.SnippetID()] = true
	}
	assert.True(t, ids["vc-bm25-1"], "expected kubernetes document in results")

	// Find with snippet filter.
	filtered, err := store.Find(ctx,
		search.WithQuery("kubernetes"),
		search.WithSnippetIDs([]string{"vc-bm25-2", "vc-bm25-3"}),
		repository.WithLimit(10),
	)
	require.NoError(t, err)
	for _, r := range filtered {
		assert.NotEqual(t, "vc-bm25-1", r.SnippetID(), "filtered search should exclude vc-bm25-1")
	}

	// Delete documents.
	err = store.DeleteBy(ctx, search.WithSnippetIDs([]string{"vc-bm25-1", "vc-bm25-2", "vc-bm25-3"}))
	require.NoError(t, err)

	// After deletion, find should return no results for deleted docs.
	afterDelete, err := store.Find(ctx,
		search.WithQuery("kubernetes pods"),
		repository.WithLimit(10),
	)
	require.NoError(t, err)
	for _, r := range afterDelete {
		assert.NotEqual(t, "vc-bm25-1", r.SnippetID(), "deleted document should not appear in results")
	}
}
