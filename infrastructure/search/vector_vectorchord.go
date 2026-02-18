package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

// SQL queries that must stay as raw SQL (extensions, indexes, catalog).
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

// TaskName represents the type of embeddings (code or text).
type TaskName string

// TaskName values.
const (
	TaskNameCode TaskName = "code"
	TaskNameText TaskName = "text"
)

// ErrVectorInitializationFailed indicates VectorChord vector initialization failed.
var ErrVectorInitializationFailed = errors.New("failed to initialize VectorChord vector repository")

// ErrDimensionMismatch indicates embedding dimension doesn't match database.
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

// VectorChordVectorStore implements search.VectorStore using VectorChord PostgreSQL extension.
type VectorChordVectorStore struct {
	repo     database.Repository[PgEmbeddingEntity, PgEmbeddingEntity]
	embedder provider.Embedder
	logger   *slog.Logger
}

// NewVectorChordVectorStore creates a new VectorChordVectorStore, eagerly
// initializing the extension, table, index, and verifying the dimension.
func NewVectorChordVectorStore(ctx context.Context, db database.Database, taskName TaskName, embedder provider.Embedder, logger *slog.Logger) (*VectorChordVectorStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	tableName := fmt.Sprintf("vectorchord_%s_embeddings", taskName)
	s := &VectorChordVectorStore{
		repo:     newPgEmbeddingRepository(db, tableName),
		embedder: embedder,
		logger:   logger,
	}

	rawDB := db.Session(ctx)

	// Create extension
	if err := rawDB.Exec(vcCreateVChordExtension).Error; err != nil {
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

	// Create table (dynamic dimension requires raw SQL)
	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding VECTOR(%d) NOT NULL
)`, tableName, dimension)
	if err := rawDB.Exec(createTableSQL).Error; err != nil {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create table: %w", err))
	}

	// Migrate index from old operator class if needed
	if err := s.migrateIndex(ctx); err != nil {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("migrate index: %w", err))
	}

	// Create index (uses vector_cosine_ops)
	indexSQL := fmt.Sprintf(vcCreateVChordIndexTemplate, tableName, tableName)
	if err := rawDB.Exec(indexSQL).Error; err != nil {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create index: %w", err))
	}

	// Verify dimension matches
	var dbDimension int
	checkDimensionSQL := fmt.Sprintf(vcCheckDimensionTemplate, tableName)
	result := rawDB.Raw(checkDimensionSQL).Scan(&dbDimension)
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
	tableName := s.repo.Table()
	db := s.repo.DB(ctx)

	var opclass string
	query := fmt.Sprintf(vcCheckIndexOpClassTemplate, tableName)
	result := db.Raw(query).Scan(&opclass)
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
		slog.String("table", tableName),
		slog.String("old_opclass", opclass),
		slog.String("new_opclass", "vector_cosine_ops"),
	)

	dropSQL := fmt.Sprintf("DROP INDEX IF EXISTS %s_idx", tableName)
	if err := db.Exec(dropSQL).Error; err != nil {
		return fmt.Errorf("drop old index: %w", err)
	}

	s.logger.Info("VectorChord index migrated — index will be recreated with vector_cosine_ops",
		slog.String("table", tableName),
	)
	return nil
}

// Index adds documents to the vector index with embeddings.
func (s *VectorChordVectorStore) Index(ctx context.Context, request search.IndexRequest) error {
	return indexDocuments(ctx, &s.repo, s.embedder, s.logger, request, pgEntityFactory)
}

// Find performs vector similarity search.
func (s *VectorChordVectorStore) Find(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	return cosineSearch(ctx, s.repo.DB(ctx), s.repo.Table(), options...)
}

// Exists checks if a snippet matching the options exists.
func (s *VectorChordVectorStore) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	return s.repo.Exists(ctx, options...)
}

// SnippetIDs returns snippet IDs matching the given options.
func (s *VectorChordVectorStore) SnippetIDs(ctx context.Context, options ...repository.Option) ([]string, error) {
	var found []string
	db := database.ApplyOptions(s.repo.DB(ctx), options...)
	err := db.Pluck("snippet_id", &found).Error
	if err != nil {
		return nil, err
	}
	return found, nil
}

// DeleteBy removes documents matching the given options.
func (s *VectorChordVectorStore) DeleteBy(ctx context.Context, options ...repository.Option) error {
	return s.repo.DeleteBy(ctx, options...)
}
