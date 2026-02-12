package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/infrastructure/provider"
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
	vcCreateVChordExtension = `CREATE EXTENSION IF NOT EXISTS vchord CASCADE`

	vcCreateVChordIndexTemplate = `
CREATE INDEX IF NOT EXISTS %s_idx
ON %s
USING vchordrq (embedding vector_l2_ops) WITH (options = $$
residual_quantization = true
[build.internal]
lists = []
$$)`

	vcInsertQueryTemplate = `
INSERT INTO %s (snippet_id, embedding)
VALUES (?, ?)
ON CONFLICT (snippet_id) DO UPDATE
SET embedding = EXCLUDED.embedding`

	vcSearchQueryTemplate = `
SELECT snippet_id, embedding <=> ? as score
FROM %s
ORDER BY score ASC
LIMIT ?`

	vcSearchQueryWithFilterTemplate = `
SELECT snippet_id, embedding <=> ? as score
FROM %s
WHERE snippet_id IN ?
ORDER BY score ASC
LIMIT ?`

	vcCheckEmbeddingExistsTemplate = `
SELECT EXISTS(SELECT 1 FROM %s WHERE snippet_id = ?)`

	vcCheckExistingIDsTemplate = `
SELECT snippet_id FROM %s WHERE snippet_id IN ?`

	vcCheckDimensionTemplate = `
SELECT a.atttypmod as dimension
FROM pg_attribute a
JOIN pg_class c ON a.attrelid = c.oid
WHERE c.relname = '%s'
AND a.attname = 'embedding'`
)

// ErrVectorInitializationFailed indicates VectorChord vector initialization failed.
var ErrVectorInitializationFailed = errors.New("failed to initialize VectorChord vector repository")

// ErrDimensionMismatch indicates embedding dimension doesn't match database.
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

// VectorChordVectorStore implements search.VectorStore using VectorChord PostgreSQL extension.
type VectorChordVectorStore struct {
	db          *gorm.DB
	embedder    provider.Embedder
	logger      *slog.Logger
	tableName   string
	initialized bool
	mu          sync.Mutex
}

// NewVectorChordVectorStore creates a new VectorChordVectorStore.
func NewVectorChordVectorStore(db *gorm.DB, taskName TaskName, embedder provider.Embedder, logger *slog.Logger) *VectorChordVectorStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &VectorChordVectorStore{
		db:        db,
		embedder:  embedder,
		logger:    logger,
		tableName: fmt.Sprintf("vectorchord_%s_embeddings", taskName),
	}
}

func (s *VectorChordVectorStore) initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	if err := s.createExtension(ctx); err != nil {
		return errors.Join(ErrVectorInitializationFailed, err)
	}

	if err := s.createTable(ctx); err != nil {
		return errors.Join(ErrVectorInitializationFailed, err)
	}

	s.initialized = true
	return nil
}

func (s *VectorChordVectorStore) createExtension(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec(vcCreateVChordExtension).Error
}

func (s *VectorChordVectorStore) createTable(ctx context.Context) error {
	// Get embedding dimension from provider
	resp, err := s.embedder.Embed(ctx, provider.NewEmbeddingRequest([]string{"dimension probe"}))
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
)`, s.tableName, dimension)

	if err := s.db.WithContext(ctx).Exec(createTableSQL).Error; err != nil {
		return err
	}

	// Create index
	indexSQL := fmt.Sprintf(vcCreateVChordIndexTemplate, s.tableName, s.tableName)
	if err := s.db.WithContext(ctx).Exec(indexSQL).Error; err != nil {
		return err
	}

	// Verify dimension matches
	var dbDimension int
	checkDimensionSQL := fmt.Sprintf(vcCheckDimensionTemplate, s.tableName)
	result := s.db.WithContext(ctx).Raw(checkDimensionSQL).Scan(&dbDimension)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	if result.RowsAffected > 0 && dbDimension != dimension {
		return fmt.Errorf("%w: database has %d, provider has %d", ErrDimensionMismatch, dbDimension, dimension)
	}

	return nil
}

func (s *VectorChordVectorStore) existingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var existingIDs []string
	query := fmt.Sprintf(vcCheckExistingIDsTemplate, s.tableName)
	err := s.db.WithContext(ctx).Raw(query, ids).Scan(&existingIDs).Error
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
func (s *VectorChordVectorStore) Index(ctx context.Context, request search.IndexRequest) error {
	if err := s.initialize(ctx); err != nil {
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

	existing, err := s.existingIDs(ctx, ids)
	if err != nil {
		return err
	}

	var toIndex []search.Document
	for _, doc := range documents {
		if _, exists := existing[doc.SnippetID()]; !exists {
			toIndex = append(toIndex, doc)
		}
	}

	if len(toIndex) == 0 {
		s.logger.Info("no new documents to index")
		return nil
	}

	// Get embeddings for documents
	texts := make([]string, len(toIndex))
	for i, doc := range toIndex {
		texts[i] = doc.Text()
	}

	embResp, err := s.embedder.Embed(ctx, provider.NewEmbeddingRequest(texts))
	if err != nil {
		return fmt.Errorf("generate embeddings: %w", err)
	}

	embeddings := embResp.Embeddings()
	if len(embeddings) != len(toIndex) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(toIndex))
	}

	// Insert documents with embeddings
	insertQuery := fmt.Sprintf(vcInsertQueryTemplate, s.tableName)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
func (s *VectorChordVectorStore) Search(ctx context.Context, request search.Request) ([]search.Result, error) {
	if err := s.initialize(ctx); err != nil {
		return nil, err
	}

	query := request.Query()
	if query == "" {
		return []search.Result{}, nil
	}

	topK := request.TopK()
	if topK <= 0 {
		topK = 10
	}

	// Get embedding for query
	embResp, err := s.embedder.Embed(ctx, provider.NewEmbeddingRequest([]string{query}))
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	embeddings := embResp.Embeddings()
	if len(embeddings) == 0 {
		return []search.Result{}, nil
	}

	queryEmbedding := formatEmbedding(embeddings[0])

	var rows []struct {
		SnippetID string  `gorm:"column:snippet_id"`
		Score     float64 `gorm:"column:score"`
	}

	snippetIDs := request.SnippetIDs()
	if len(snippetIDs) > 0 {
		searchSQL := fmt.Sprintf(vcSearchQueryWithFilterTemplate, s.tableName)
		err = s.db.WithContext(ctx).Raw(searchSQL, queryEmbedding, snippetIDs, topK).Scan(&rows).Error
	} else {
		searchSQL := fmt.Sprintf(vcSearchQueryTemplate, s.tableName)
		err = s.db.WithContext(ctx).Raw(searchSQL, queryEmbedding, topK).Scan(&rows).Error
	}

	if err != nil {
		return nil, err
	}

	results := make([]search.Result, len(rows))
	for i, row := range rows {
		// VectorChord returns cosine distance (0 = identical, 2 = opposite)
		// Convert to similarity score (1 - distance/2 for 0-1 range)
		similarity := 1.0 - row.Score/2.0
		results[i] = search.NewResult(row.SnippetID, similarity)
	}

	return results, nil
}

// HasEmbedding checks if a snippet has an embedding of the given type.
func (s *VectorChordVectorStore) HasEmbedding(ctx context.Context, snippetID string, embeddingType snippet.EmbeddingType) (bool, error) {
	if err := s.initialize(ctx); err != nil {
		return false, err
	}

	// Note: embeddingType is not used here because VectorChord uses separate tables per task
	_ = embeddingType

	var exists bool
	query := fmt.Sprintf(vcCheckEmbeddingExistsTemplate, s.tableName)
	err := s.db.WithContext(ctx).Raw(query, snippetID).Scan(&exists).Error
	if err != nil {
		return false, err
	}

	return exists, nil
}

// Delete removes documents from the vector index.
func (s *VectorChordVectorStore) Delete(ctx context.Context, request search.DeleteRequest) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}

	ids := request.SnippetIDs()
	if len(ids) == 0 {
		return nil
	}

	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE snippet_id IN ?", s.tableName)
	return s.db.WithContext(ctx).Exec(deleteSQL, ids).Error
}

// formatEmbedding formats a float slice as a PostgreSQL vector string.
func formatEmbedding(embedding []float64) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
