package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/infrastructure/provider"
	"gorm.io/gorm"
)

// SQL queries for pgvector.
const (
	pgvCreateExtension = `CREATE EXTENSION IF NOT EXISTS vector`

	pgvCreateTableTemplate = `
CREATE TABLE IF NOT EXISTS %s (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding VECTOR(%d) NOT NULL
)`

	pgvCreateIndexTemplate = `
CREATE INDEX IF NOT EXISTS %s_idx
ON %s
USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100)`

	pgvInsertQueryTemplate = `
INSERT INTO %s (snippet_id, embedding)
VALUES (?, ?)
ON CONFLICT (snippet_id) DO UPDATE
SET embedding = EXCLUDED.embedding`

	pgvSearchQueryTemplate = `
SELECT snippet_id, embedding <=> ? as score
FROM %s
ORDER BY score ASC
LIMIT ?`

	pgvSearchQueryWithFilterTemplate = `
SELECT snippet_id, embedding <=> ? as score
FROM %s
WHERE snippet_id IN ?
ORDER BY score ASC
LIMIT ?`

	pgvCheckEmbeddingExistsTemplate = `
SELECT EXISTS(SELECT 1 FROM %s WHERE snippet_id = ?)`

	pgvCheckExistingIDsTemplate = `
SELECT snippet_id FROM %s WHERE snippet_id IN ?`

	pgvCheckDimensionTemplate = `
SELECT a.atttypmod as dimension
FROM pg_attribute a
JOIN pg_class c ON a.attrelid = c.oid
WHERE c.relname = '%s'
AND a.attname = 'embedding'`
)

// ErrPgvectorInitializationFailed indicates pgvector initialization failed.
var ErrPgvectorInitializationFailed = errors.New("failed to initialize pgvector store")

// PgvectorStore implements search.VectorStore using PostgreSQL pgvector extension.
type PgvectorStore struct {
	db          *gorm.DB
	embedder    provider.Embedder
	logger      *slog.Logger
	tableName   string
	initialized bool
	mu          sync.Mutex
}

// NewPgvectorStore creates a new PgvectorStore.
func NewPgvectorStore(db *gorm.DB, taskName TaskName, embedder provider.Embedder, logger *slog.Logger) *PgvectorStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &PgvectorStore{
		db:        db,
		embedder:  embedder,
		logger:    logger,
		tableName: fmt.Sprintf("pgvector_%s_embeddings", taskName),
	}
}

func (s *PgvectorStore) initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	if err := s.createExtension(ctx); err != nil {
		return errors.Join(ErrPgvectorInitializationFailed, err)
	}

	if err := s.createTable(ctx); err != nil {
		return errors.Join(ErrPgvectorInitializationFailed, err)
	}

	s.initialized = true
	return nil
}

func (s *PgvectorStore) createExtension(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec(pgvCreateExtension).Error
}

func (s *PgvectorStore) createTable(ctx context.Context) error {
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
	createTableSQL := fmt.Sprintf(pgvCreateTableTemplate, s.tableName, dimension)
	if err := s.db.WithContext(ctx).Exec(createTableSQL).Error; err != nil {
		return err
	}

	// Create index (ignore errors if index already exists with different parameters)
	indexSQL := fmt.Sprintf(pgvCreateIndexTemplate, s.tableName, s.tableName)
	if err := s.db.WithContext(ctx).Exec(indexSQL).Error; err != nil {
		s.logger.Warn("failed to create index (may already exist)", "error", err)
	}

	// Verify dimension matches
	var dbDimension int
	checkDimensionSQL := fmt.Sprintf(pgvCheckDimensionTemplate, s.tableName)
	result := s.db.WithContext(ctx).Raw(checkDimensionSQL).Scan(&dbDimension)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	if result.RowsAffected > 0 && dbDimension != dimension {
		return fmt.Errorf("%w: database has %d, provider has %d", ErrDimensionMismatch, dbDimension, dimension)
	}

	return nil
}

func (s *PgvectorStore) existingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var existingIDs []string
	query := fmt.Sprintf(pgvCheckExistingIDsTemplate, s.tableName)
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
func (s *PgvectorStore) Index(ctx context.Context, request search.IndexRequest) error {
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
	insertQuery := fmt.Sprintf(pgvInsertQueryTemplate, s.tableName)
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
func (s *PgvectorStore) Search(ctx context.Context, request search.Request) ([]search.Result, error) {
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
		searchSQL := fmt.Sprintf(pgvSearchQueryWithFilterTemplate, s.tableName)
		err = s.db.WithContext(ctx).Raw(searchSQL, queryEmbedding, snippetIDs, topK).Scan(&rows).Error
	} else {
		searchSQL := fmt.Sprintf(pgvSearchQueryTemplate, s.tableName)
		err = s.db.WithContext(ctx).Raw(searchSQL, queryEmbedding, topK).Scan(&rows).Error
	}

	if err != nil {
		return nil, err
	}

	results := make([]search.Result, len(rows))
	for i, row := range rows {
		// pgvector <=> returns cosine distance (0 = identical, 2 = opposite)
		// Convert to similarity score (1 - distance/2 for 0-1 range)
		similarity := 1.0 - row.Score/2.0
		results[i] = search.NewResult(row.SnippetID, similarity)
	}

	return results, nil
}

// HasEmbedding checks if a snippet has an embedding of the given type.
func (s *PgvectorStore) HasEmbedding(ctx context.Context, snippetID string, embeddingType snippet.EmbeddingType) (bool, error) {
	if err := s.initialize(ctx); err != nil {
		return false, err
	}

	// Note: embeddingType is not used here because pgvector uses separate tables per task
	_ = embeddingType

	var exists bool
	query := fmt.Sprintf(pgvCheckEmbeddingExistsTemplate, s.tableName)
	err := s.db.WithContext(ctx).Raw(query, snippetID).Scan(&exists).Error
	if err != nil {
		return false, err
	}

	return exists, nil
}

// Delete removes documents from the vector index.
func (s *PgvectorStore) Delete(ctx context.Context, request search.DeleteRequest) error {
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
