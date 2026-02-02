// Package vector provides vector similarity search implementations.
package vector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/provider"
	"gorm.io/gorm"
)

// TaskName represents the type of embeddings (code or text).
type TaskName string

// TaskName values.
const (
	TaskNameCode TaskName = "code"
	TaskNameText TaskName = "text"
)

// SQL queries for VectorChord vector search.
const (
	createVChordExtension = `CREATE EXTENSION IF NOT EXISTS vchord CASCADE`

	createVChordIndexTemplate = `
CREATE INDEX IF NOT EXISTS %s_idx
ON %s
USING vchordrq (embedding vector_l2_ops) WITH (options = $$
residual_quantization = true
[build.internal]
lists = []
$$)`

	insertQueryTemplate = `
INSERT INTO %s (snippet_id, embedding)
VALUES (?, ?)
ON CONFLICT (snippet_id) DO UPDATE
SET embedding = EXCLUDED.embedding`

	searchQueryTemplate = `
SELECT snippet_id, embedding <=> ? as score
FROM %s
ORDER BY score ASC
LIMIT ?`

	searchQueryWithFilterTemplate = `
SELECT snippet_id, embedding <=> ? as score
FROM %s
WHERE snippet_id IN ?
ORDER BY score ASC
LIMIT ?`

	checkEmbeddingExistsTemplate = `
SELECT EXISTS(SELECT 1 FROM %s WHERE snippet_id = ?)`

	checkExistingIDsTemplate = `
SELECT snippet_id FROM %s WHERE snippet_id IN ?`

	checkDimensionTemplate = `
SELECT a.atttypmod as dimension
FROM pg_attribute a
JOIN pg_class c ON a.attrelid = c.oid
WHERE c.relname = '%s'
AND a.attname = 'embedding'`
)

// ErrInitializationFailed indicates VectorChord initialization failed.
var ErrInitializationFailed = errors.New("failed to initialize VectorChord vector repository")

// ErrDimensionMismatch indicates embedding dimension doesn't match database.
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

// VectorChordRepository implements VectorSearchRepository using VectorChord PostgreSQL extension.
type VectorChordRepository struct {
	db               *gorm.DB
	embedder         provider.Embedder
	logger           *slog.Logger
	tableName        string
	initialized      bool
	mu               sync.Mutex
}

// NewVectorChordRepository creates a new VectorChordRepository.
func NewVectorChordRepository(db *gorm.DB, taskName TaskName, embedder provider.Embedder, logger *slog.Logger) *VectorChordRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &VectorChordRepository{
		db:        db,
		embedder:  embedder,
		logger:    logger,
		tableName: fmt.Sprintf("vectorchord_%s_embeddings", taskName),
	}
}

// initialize sets up the VectorChord environment.
func (r *VectorChordRepository) initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return nil
	}

	if err := r.createExtension(ctx); err != nil {
		return errors.Join(ErrInitializationFailed, err)
	}

	if err := r.createTable(ctx); err != nil {
		return errors.Join(ErrInitializationFailed, err)
	}

	r.initialized = true
	return nil
}

func (r *VectorChordRepository) createExtension(ctx context.Context) error {
	return r.db.WithContext(ctx).Exec(createVChordExtension).Error
}

func (r *VectorChordRepository) createTable(ctx context.Context) error {
	// Get embedding dimension from provider
	resp, err := r.embedder.Embed(ctx, provider.NewEmbeddingRequest([]string{"dimension probe"}))
	if err != nil {
		return fmt.Errorf("probe embedding dimension: %w", err)
	}

	embeddings := resp.Embeddings()
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return errors.New("failed to obtain embedding dimension from provider")
	}

	dimension := len(embeddings[0])

	// Create table with correct vector dimension
	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding VECTOR(%d) NOT NULL
)`, r.tableName, dimension)

	if err := r.db.WithContext(ctx).Exec(createTableSQL).Error; err != nil {
		return err
	}

	// Create index
	indexSQL := fmt.Sprintf(createVChordIndexTemplate, r.tableName, r.tableName)
	if err := r.db.WithContext(ctx).Exec(indexSQL).Error; err != nil {
		return err
	}

	// Verify dimension matches
	var dbDimension int
	checkDimensionSQL := fmt.Sprintf(checkDimensionTemplate, r.tableName)
	result := r.db.WithContext(ctx).Raw(checkDimensionSQL).Scan(&dbDimension)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	if result.RowsAffected > 0 && dbDimension != dimension {
		return fmt.Errorf("%w: database has %d, provider has %d", ErrDimensionMismatch, dbDimension, dimension)
	}

	return nil
}

func (r *VectorChordRepository) existingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var existingIDs []string
	query := fmt.Sprintf(checkExistingIDsTemplate, r.tableName)
	err := r.db.WithContext(ctx).Raw(query, ids).Scan(&existingIDs).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		result[id] = struct{}{}
	}
	return result, nil
}

// Index adds documents to the vector index with embeddings.
func (r *VectorChordRepository) Index(ctx context.Context, request domain.IndexRequest) error {
	if err := r.initialize(ctx); err != nil {
		return err
	}

	documents := request.Documents()
	if len(documents) == 0 {
		return nil
	}

	// Filter out already indexed documents
	ids := make([]string, len(documents))
	for i, doc := range documents {
		ids[i] = doc.SnippetID()
	}

	existing, err := r.existingIDs(ctx, ids)
	if err != nil {
		return err
	}

	var toIndex []domain.Document
	for _, doc := range documents {
		if _, exists := existing[doc.SnippetID()]; !exists {
			toIndex = append(toIndex, doc)
		}
	}

	if len(toIndex) == 0 {
		r.logger.Info("no new documents to index")
		return nil
	}

	// Get embeddings for documents
	texts := make([]string, len(toIndex))
	for i, doc := range toIndex {
		texts[i] = doc.Text()
	}

	embResp, err := r.embedder.Embed(ctx, provider.NewEmbeddingRequest(texts))
	if err != nil {
		return fmt.Errorf("generate embeddings: %w", err)
	}

	embeddings := embResp.Embeddings()
	if len(embeddings) != len(toIndex) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(toIndex))
	}

	// Insert documents with embeddings
	insertQuery := fmt.Sprintf(insertQueryTemplate, r.tableName)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, doc := range toIndex {
			embeddingStr := formatEmbedding(embeddings[i])
			if err := tx.Exec(insertQuery, doc.SnippetID(), embeddingStr).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Search performs vector similarity search.
func (r *VectorChordRepository) Search(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error) {
	if err := r.initialize(ctx); err != nil {
		return nil, err
	}

	query := request.Query()
	if query == "" {
		return []domain.SearchResult{}, nil
	}

	topK := request.TopK()
	if topK <= 0 {
		topK = 10
	}

	// Get embedding for query
	embResp, err := r.embedder.Embed(ctx, provider.NewEmbeddingRequest([]string{query}))
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	embeddings := embResp.Embeddings()
	if len(embeddings) == 0 {
		return []domain.SearchResult{}, nil
	}

	queryEmbedding := formatEmbedding(embeddings[0])

	var rows []struct {
		SnippetID string  `gorm:"column:snippet_id"`
		Score     float64 `gorm:"column:score"`
	}

	snippetIDs := request.SnippetIDs()
	if len(snippetIDs) > 0 {
		searchSQL := fmt.Sprintf(searchQueryWithFilterTemplate, r.tableName)
		err = r.db.WithContext(ctx).Raw(searchSQL, queryEmbedding, snippetIDs, topK).Scan(&rows).Error
	} else {
		searchSQL := fmt.Sprintf(searchQueryTemplate, r.tableName)
		err = r.db.WithContext(ctx).Raw(searchSQL, queryEmbedding, topK).Scan(&rows).Error
	}

	if err != nil {
		return nil, err
	}

	results := make([]domain.SearchResult, len(rows))
	for i, row := range rows {
		// VectorChord returns cosine distance (0 = identical, 2 = opposite)
		// Convert to similarity score (1 - distance/2 for 0-1 range)
		similarity := 1.0 - row.Score/2.0
		results[i] = domain.NewSearchResult(row.SnippetID, similarity)
	}

	return results, nil
}

// HasEmbedding checks if a snippet has an embedding of the given type.
func (r *VectorChordRepository) HasEmbedding(ctx context.Context, snippetID string, embeddingType indexing.EmbeddingType) (bool, error) {
	if err := r.initialize(ctx); err != nil {
		return false, err
	}

	// Note: embeddingType is not used here because VectorChord uses separate tables per task
	_ = embeddingType

	var exists bool
	query := fmt.Sprintf(checkEmbeddingExistsTemplate, r.tableName)
	err := r.db.WithContext(ctx).Raw(query, snippetID).Scan(&exists).Error
	if err != nil {
		return false, err
	}

	return exists, nil
}

// Delete removes documents from the vector index.
func (r *VectorChordRepository) Delete(ctx context.Context, request domain.DeleteRequest) error {
	if err := r.initialize(ctx); err != nil {
		return err
	}

	ids := request.SnippetIDs()
	if len(ids) == 0 {
		return nil
	}

	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE snippet_id IN ?", r.tableName)
	return r.db.WithContext(ctx).Exec(deleteSQL, ids).Error
}

// formatEmbedding formats a float slice as a PostgreSQL vector string.
func formatEmbedding(embedding []float64) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
