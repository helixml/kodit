package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/helixml/kodit/domain/search"
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
USING vchordrq (embedding vector_cosine_ops) WITH (options = $$
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

	vcCheckIndexOpClassTemplate = `
SELECT opcname FROM pg_index i
JOIN pg_opclass o ON o.oid = i.indclass[0]
JOIN pg_class c ON c.oid = i.indexrelid
WHERE c.relname = '%s_idx'`
)

// ErrVectorInitializationFailed indicates VectorChord vector initialization failed.
var ErrVectorInitializationFailed = errors.New("failed to initialize VectorChord vector repository")

// ErrDimensionMismatch indicates embedding dimension doesn't match database.
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

// VectorChordVectorStore implements search.VectorStore using VectorChord PostgreSQL extension.
type VectorChordVectorStore struct {
	db        *gorm.DB
	embedder  provider.Embedder
	logger    *slog.Logger
	tableName string
}

// NewVectorChordVectorStore creates a new VectorChordVectorStore, eagerly
// initializing the extension, table, index, and verifying the dimension.
func NewVectorChordVectorStore(ctx context.Context, db *gorm.DB, taskName TaskName, embedder provider.Embedder, logger *slog.Logger) (*VectorChordVectorStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	s := &VectorChordVectorStore{
		db:        db,
		embedder:  embedder,
		logger:    logger,
		tableName: fmt.Sprintf("vectorchord_%s_embeddings", taskName),
	}

	// Create extension
	if err := db.WithContext(ctx).Exec(vcCreateVChordExtension).Error; err != nil {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create extension: %w", err))
	}

	// Probe embedding dimension
	resp, err := embedder.Embed(ctx, provider.NewEmbeddingRequest([]string{"dimension probe"}))
	if err != nil {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("probe embedding dimension: %w", err))
	}
	probeEmbeddings := resp.Embeddings()
	if len(probeEmbeddings) == 0 || len(probeEmbeddings[0]) == 0 {
		return nil, errors.Join(ErrVectorInitializationFailed, errors.New("failed to obtain embedding dimension from provider"))
	}
	dimension := len(probeEmbeddings[0])

	// Create table
	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding VECTOR(%d) NOT NULL
)`, s.tableName, dimension)
	if err := db.WithContext(ctx).Exec(createTableSQL).Error; err != nil {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create table: %w", err))
	}

	// Migrate index from old operator class if needed
	if err := s.migrateIndex(ctx); err != nil {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("migrate index: %w", err))
	}

	// Create index (uses vector_cosine_ops)
	indexSQL := fmt.Sprintf(vcCreateVChordIndexTemplate, s.tableName, s.tableName)
	if err := db.WithContext(ctx).Exec(indexSQL).Error; err != nil {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create index: %w", err))
	}

	// Verify dimension matches
	var dbDimension int
	checkDimensionSQL := fmt.Sprintf(vcCheckDimensionTemplate, s.tableName)
	result := db.WithContext(ctx).Raw(checkDimensionSQL).Scan(&dbDimension)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("check dimension: %w", result.Error))
	}
	if result.RowsAffected > 0 && dbDimension != dimension {
		return nil, fmt.Errorf("%w: database has %d dimensions, provider has %d — if you switched embedding providers, drop the embedding tables and re-index", ErrDimensionMismatch, dbDimension, dimension)
	}

	return s, nil
}

// migrateIndex drops a VectorChord index that was created with the wrong
// operator class (e.g. vector_l2_ops instead of vector_cosine_ops). The
// embedding data is preserved — only the index is rebuilt.
func (s *VectorChordVectorStore) migrateIndex(ctx context.Context) error {
	var opclass string
	query := fmt.Sprintf(vcCheckIndexOpClassTemplate, s.tableName)
	result := s.db.WithContext(ctx).Raw(query).Scan(&opclass)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check index opclass: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil // index does not exist yet
	}
	if opclass == "vector_cosine_ops" {
		return nil // already correct
	}

	s.logger.Warn("VectorChord index uses wrong operator class, dropping index for recreation",
		slog.String("table", s.tableName),
		slog.String("old_opclass", opclass),
		slog.String("new_opclass", "vector_cosine_ops"),
	)

	dropSQL := fmt.Sprintf("DROP INDEX IF EXISTS %s_idx", s.tableName)
	if err := s.db.WithContext(ctx).Exec(dropSQL).Error; err != nil {
		return fmt.Errorf("drop old index: %w", err)
	}

	s.logger.Info("VectorChord index migrated — index will be recreated with vector_cosine_ops",
		slog.String("table", s.tableName),
	)
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
func (s *VectorChordVectorStore) HasEmbedding(ctx context.Context, snippetID string, embeddingType search.EmbeddingType) (bool, error) {
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
		parts[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
