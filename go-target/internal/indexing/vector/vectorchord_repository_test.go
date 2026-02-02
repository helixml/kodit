package vector

import (
	"context"
	"testing"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Note: VectorChord requires PostgreSQL extensions (vchord)
// that are not available in SQLite. These tests cover the basic structure and
// non-PostgreSQL-specific behavior. Full integration tests require PostgreSQL.

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
		for j := range embedding {
			embedding[j] = float64(i+j) / float64(f.dimension)
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

func TestNewVectorChordRepository(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	repo := NewVectorChordRepository(db, TaskNameCode, embedder, nil)

	assert.NotNil(t, repo)
	assert.NotNil(t, repo.logger)
	assert.False(t, repo.initialized)
	assert.Equal(t, "vectorchord_code_embeddings", repo.tableName)
}

func TestNewVectorChordRepository_TextTask(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	repo := NewVectorChordRepository(db, TaskNameText, embedder, nil)

	assert.Equal(t, "vectorchord_text_embeddings", repo.tableName)
}

func TestVectorChordRepository_Index_EmptyDocuments(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	repo := NewVectorChordRepository(db, TaskNameCode, embedder, nil)
	// Mark as initialized to skip PostgreSQL-specific initialization
	repo.initialized = true
	ctx := context.Background()

	// Empty documents should succeed without error
	err := repo.Index(ctx, domain.NewIndexRequest([]domain.Document{}))
	require.NoError(t, err)
}

func TestVectorChordRepository_Search_EmptyQuery(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	repo := NewVectorChordRepository(db, TaskNameCode, embedder, nil)
	repo.initialized = true
	ctx := context.Background()

	// Empty query should return empty results
	results, err := repo.Search(ctx, domain.NewSearchRequest("", 10, nil))
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestVectorChordRepository_Delete_EmptyIDs(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	repo := NewVectorChordRepository(db, TaskNameCode, embedder, nil)
	repo.initialized = true
	ctx := context.Background()

	// Empty IDs should succeed without error
	err := repo.Delete(ctx, domain.NewDeleteRequest([]string{}))
	require.NoError(t, err)
}

func TestVectorChordRepository_HasEmbedding_NotInitialized(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	repo := NewVectorChordRepository(db, TaskNameCode, embedder, nil)
	ctx := context.Background()

	// Without initialization, operations should attempt to initialize
	// which will fail with SQLite (no VectorChord extensions)
	_, err := repo.HasEmbedding(ctx, "snippet1", indexing.EmbeddingTypeCode)

	// Should fail because SQLite doesn't have VectorChord extensions
	assert.Error(t, err)
}

func TestVectorChordRepository_InitializationIdempotent(t *testing.T) {
	db := testDB(t)
	embedder := newFakeEmbedder(384)
	repo := NewVectorChordRepository(db, TaskNameCode, embedder, nil)
	repo.initialized = true // Manually mark as initialized

	ctx := context.Background()

	// These operations should not cause re-initialization attempts
	results, err := repo.Search(ctx, domain.NewSearchRequest("", 10, nil))
	require.NoError(t, err)
	assert.Empty(t, results)

	err = repo.Delete(ctx, domain.NewDeleteRequest([]string{}))
	require.NoError(t, err)
}

func TestFormatEmbedding(t *testing.T) {
	embedding := []float64{0.1, 0.2, 0.3}
	result := formatEmbedding(embedding)

	assert.Contains(t, result, "[")
	assert.Contains(t, result, "]")
	assert.Contains(t, result, "0.1")
	assert.Contains(t, result, "0.2")
	assert.Contains(t, result, "0.3")
}

func TestTaskNameValues(t *testing.T) {
	assert.Equal(t, TaskName("code"), TaskNameCode)
	assert.Equal(t, TaskName("text"), TaskNameText)
}
