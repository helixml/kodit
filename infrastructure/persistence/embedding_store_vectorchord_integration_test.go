//go:build integration

package persistence

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVectorChordEmbeddingStore_MissingTable verifies that Find, DeleteBy,
// Exists, and Search return empty results (not errors) when the backing table
// has not been created yet by SaveAll.
//
//	VECTORCHORD_TEST_URL="postgresql://postgres:mysecretpassword@localhost:5434/kodit" go test -v -run TestVectorChordEmbeddingStore_MissingTable ./infrastructure/persistence/
func TestVectorChordEmbeddingStore_MissingTable(t *testing.T) {
	dsn := os.Getenv("VECTORCHORD_TEST_URL")
	if dsn == "" {
		t.Skip("VECTORCHORD_TEST_URL not set")
	}

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t))

	db, err := database.NewDatabase(ctx, dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Use a unique task name so the table definitely doesn't exist.
	taskName := TaskName("test_missing_table")
	tableName := fmt.Sprintf("vectorchord_%s_embeddings", taskName)

	// Ensure the table does not exist before we start.
	db.Session(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))

	store := NewVectorChordEmbeddingStore(db, taskName, nil, logger)

	t.Run("Find returns empty on missing table", func(t *testing.T) {
		results, err := store.Find(ctx, search.WithSnippetIDs([]string{"1", "2", "3"}))
		assert.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("DeleteBy succeeds on missing table", func(t *testing.T) {
		err := store.DeleteBy(ctx, search.WithSnippetIDs([]string{"1"}))
		assert.NoError(t, err)
	})

	t.Run("Exists returns false on missing table", func(t *testing.T) {
		exists, err := store.Exists(ctx, search.WithSnippetIDs([]string{"1"}))
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Search returns empty on missing table", func(t *testing.T) {
		results, err := store.Search(ctx)
		assert.NoError(t, err)
		assert.Empty(t, results)
	})

	// Clean up just in case.
	db.Session(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
}

// TestVectorChordEmbeddingStore_SaveAllCreatesTable verifies that after
// SaveAll creates the table, subsequent Find/DeleteBy/Exists/Search calls
// work correctly.
//
//	VECTORCHORD_TEST_URL="postgresql://postgres:mysecretpassword@localhost:5434/kodit" go test -v -run TestVectorChordEmbeddingStore_SaveAllCreatesTable ./infrastructure/persistence/
func TestVectorChordEmbeddingStore_SaveAllCreatesTable(t *testing.T) {
	dsn := os.Getenv("VECTORCHORD_TEST_URL")
	if dsn == "" {
		t.Skip("VECTORCHORD_TEST_URL not set")
	}

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t))

	db, err := database.NewDatabase(ctx, dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	taskName := TaskName("test_saveall_creates")
	tableName := fmt.Sprintf("vectorchord_%s_embeddings", taskName)

	// Clean slate.
	db.Session(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))

	store := NewVectorChordEmbeddingStore(db, taskName, nil, logger)

	// Confirm Find returns empty before any writes.
	results, err := store.Find(ctx, search.WithSnippetIDs([]string{"1"}))
	require.NoError(t, err)
	assert.Empty(t, results)

	// SaveAll should create the table and insert data.
	embeddings := []search.Embedding{
		search.NewEmbedding("1", []float64{0.1, 0.2, 0.3, 0.4}),
		search.NewEmbedding("2", []float64{0.5, 0.6, 0.7, 0.8}),
	}
	err = store.SaveAll(ctx, embeddings)
	require.NoError(t, err)

	// Find should now return data.
	results, err = store.Find(ctx, search.WithSnippetIDs([]string{"1", "2"}))
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Exists should return true.
	exists, err := store.Exists(ctx, search.WithSnippetIDs([]string{"1"}))
	require.NoError(t, err)
	assert.True(t, exists)

	// Search should work.
	searchResults, err := store.Search(ctx,
		search.WithEmbedding([]float64{0.1, 0.2, 0.3, 0.4}),
		repository.WithLimit(10),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, searchResults)

	// DeleteBy should work.
	err = store.DeleteBy(ctx, search.WithSnippetIDs([]string{"1"}))
	require.NoError(t, err)

	// Verify deletion.
	results, err = store.Find(ctx, search.WithSnippetIDs([]string{"1"}))
	require.NoError(t, err)
	assert.Empty(t, results)

	// Clean up.
	db.Session(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
}

// TestVectorChordEmbeddingStore_ExistingTable verifies that when the table
// already exists at construction time, the flag is set and all operations
// work immediately.
//
//	VECTORCHORD_TEST_URL="postgresql://postgres:mysecretpassword@localhost:5434/kodit" go test -v -run TestVectorChordEmbeddingStore_ExistingTable ./infrastructure/persistence/
func TestVectorChordEmbeddingStore_ExistingTable(t *testing.T) {
	dsn := os.Getenv("VECTORCHORD_TEST_URL")
	if dsn == "" {
		t.Skip("VECTORCHORD_TEST_URL not set")
	}

	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t))

	db, err := database.NewDatabase(ctx, dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	taskName := TaskName("test_existing_table")
	tableName := fmt.Sprintf("vectorchord_%s_embeddings", taskName)

	// Pre-create the table (simulates a previous SaveAll run).
	db.Session(ctx).Exec("CREATE EXTENSION IF NOT EXISTS vchord CASCADE")
	db.Session(ctx).Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			snippet_id VARCHAR(255) NOT NULL UNIQUE,
			embedding VECTOR(4) NOT NULL
		)`, tableName))
	db.Session(ctx).Exec(fmt.Sprintf(
		"INSERT INTO %s (snippet_id, embedding) VALUES ('99', '[0.1,0.2,0.3,0.4]') ON CONFLICT DO NOTHING", tableName))

	store := NewVectorChordEmbeddingStore(db, taskName, nil, logger)

	// tableReady should be true immediately — Find should work.
	results, err := store.Find(ctx, search.WithSnippetIDs([]string{"99"}))
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Clean up.
	db.Session(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
}
