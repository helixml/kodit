package bm25

import (
	"context"
	"testing"

	"github.com/helixml/kodit/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Note: VectorChord requires PostgreSQL extensions (vchord, pg_tokenizer, vchord_bm25)
// that are not available in SQLite. These tests cover the basic structure and
// non-PostgreSQL-specific behavior. Full integration tests require PostgreSQL.

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	return db
}

func TestNewVectorChordRepository(t *testing.T) {
	db := testDB(t)
	repo := NewVectorChordRepository(db, nil)

	assert.NotNil(t, repo)
	assert.NotNil(t, repo.logger)
	assert.False(t, repo.initialized)
}

func TestVectorChordRepository_Index_EmptyDocuments(t *testing.T) {
	db := testDB(t)
	repo := NewVectorChordRepository(db, nil)
	// Mark as initialized to skip PostgreSQL-specific initialization
	repo.initialized = true
	ctx := context.Background()

	// Empty documents should succeed without error
	err := repo.Index(ctx, domain.NewIndexRequest([]domain.Document{}))
	require.NoError(t, err)
}

func TestVectorChordRepository_Index_InvalidDocuments(t *testing.T) {
	db := testDB(t)
	repo := NewVectorChordRepository(db, nil)
	repo.initialized = true
	ctx := context.Background()

	// Documents with empty snippet_id or text should be filtered out
	documents := []domain.Document{
		domain.NewDocument("", "some text"),      // Empty snippet_id
		domain.NewDocument("id1", ""),            // Empty text
		domain.NewDocument("", ""),               // Both empty
	}

	// Should succeed but log warning about empty corpus
	err := repo.Index(ctx, domain.NewIndexRequest(documents))
	require.NoError(t, err)
}

func TestVectorChordRepository_Search_EmptyQuery(t *testing.T) {
	db := testDB(t)
	repo := NewVectorChordRepository(db, nil)
	repo.initialized = true
	ctx := context.Background()

	// Empty query should return empty results
	results, err := repo.Search(ctx, domain.NewSearchRequest("", 10, nil))
	require.NoError(t, err)
	assert.Empty(t, results)

	// Note: Non-empty queries require PostgreSQL with VectorChord extensions
	// so we can't test those with SQLite
}

func TestVectorChordRepository_Delete_EmptyIDs(t *testing.T) {
	db := testDB(t)
	repo := NewVectorChordRepository(db, nil)
	repo.initialized = true
	ctx := context.Background()

	// Empty IDs should succeed without error
	err := repo.Delete(ctx, domain.NewDeleteRequest([]string{}))
	require.NoError(t, err)
}

func TestVectorChordRepository_NotInitialized(t *testing.T) {
	db := testDB(t)
	repo := NewVectorChordRepository(db, nil)
	ctx := context.Background()

	// Without initialization, operations should attempt to initialize
	// which will fail with SQLite (no VectorChord extensions)
	err := repo.Index(ctx, domain.NewIndexRequest([]domain.Document{
		domain.NewDocument("id1", "some text"),
	}))

	// Should fail because SQLite doesn't have VectorChord extensions
	assert.Error(t, err)
}

func TestVectorChordRepository_InitializationIdempotent(t *testing.T) {
	db := testDB(t)
	repo := NewVectorChordRepository(db, nil)
	repo.initialized = true // Manually mark as initialized

	// Multiple operations should not try to re-initialize
	ctx := context.Background()

	// These operations should not cause re-initialization attempts
	_, err := repo.Search(ctx, domain.NewSearchRequest("", 10, nil))
	require.NoError(t, err)

	err = repo.Delete(ctx, domain.NewDeleteRequest([]string{}))
	require.NoError(t, err)
}
