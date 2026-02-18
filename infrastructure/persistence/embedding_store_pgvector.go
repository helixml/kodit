package persistence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SQL queries specific to pgvector (extensions, indexes, catalog).
const (
	pgvCreateExtension = `CREATE EXTENSION IF NOT EXISTS vector`

	pgvCreateIndexTemplate = `
CREATE INDEX IF NOT EXISTS %s_idx
ON %s
USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100)`

	pgvCheckDimensionTemplate = `
SELECT a.atttypmod as dimension
FROM pg_attribute a
JOIN pg_class c ON a.attrelid = c.oid
WHERE c.relname = '%s'
AND a.attname = 'embedding'`
)

// ErrPgvectorInitializationFailed indicates pgvector initialization failed.
var ErrPgvectorInitializationFailed = errors.New("failed to initialize pgvector store")

// PgvectorEmbeddingStore implements search.EmbeddingStore using PostgreSQL pgvector extension.
type PgvectorEmbeddingStore struct {
	repo   database.Repository[search.Embedding, PgEmbeddingModel]
	logger *slog.Logger
}

// NewPgvectorEmbeddingStore creates a new PgvectorEmbeddingStore, eagerly
// initializing the extension, table, index, and verifying the dimension.
func NewPgvectorEmbeddingStore(ctx context.Context, db database.Database, taskName TaskName, dimension int, logger *slog.Logger) (*PgvectorEmbeddingStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	tableName := fmt.Sprintf("pgvector_%s_embeddings", taskName)
	s := &PgvectorEmbeddingStore{
		repo: database.NewRepositoryForTable[search.Embedding, PgEmbeddingModel](
			db, pgEmbeddingMapper{}, "embedding", tableName,
		),
		logger: logger,
	}

	rawDB := db.Session(ctx)

	// Create extension
	if err := rawDB.Exec(pgvCreateExtension).Error; err != nil {
		return nil, errors.Join(ErrPgvectorInitializationFailed, fmt.Errorf("create extension: %w", err))
	}

	// Create table with correct vector dimension (dynamic dimension requires raw SQL)
	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding VECTOR(%d) NOT NULL
)`, tableName, dimension)
	if err := rawDB.Exec(createTableSQL).Error; err != nil {
		return nil, errors.Join(ErrPgvectorInitializationFailed, fmt.Errorf("create table: %w", err))
	}

	// Create index (ignore errors if index already exists with different parameters)
	indexSQL := fmt.Sprintf(pgvCreateIndexTemplate, tableName, tableName)
	if err := rawDB.Exec(indexSQL).Error; err != nil {
		logger.Warn("failed to create index (may already exist)", "error", err)
	}

	// Verify dimension matches
	var dbDimension int
	checkDimensionSQL := fmt.Sprintf(pgvCheckDimensionTemplate, tableName)
	result := rawDB.Raw(checkDimensionSQL).Scan(&dbDimension)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.Join(ErrPgvectorInitializationFailed, fmt.Errorf("check dimension: %w", result.Error))
	}

	if result.RowsAffected > 0 && dbDimension != dimension {
		return nil, fmt.Errorf("%w: database has %d, provider has %d", ErrDimensionMismatch, dbDimension, dimension)
	}

	return s, nil
}

// SaveAll persists pre-computed embeddings using upsert.
func (s *PgvectorEmbeddingStore) SaveAll(ctx context.Context, embeddings []search.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	tableName := s.repo.Table()
	db := s.repo.DB(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		for _, emb := range embeddings {
			model := PgEmbeddingModel{
				SnippetID: emb.SnippetID(),
				Embedding: database.NewPgVector(emb.Vector()),
			}
			err := tx.Table(tableName).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "snippet_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"embedding"}),
			}).Create(&model).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Find performs vector similarity search.
func (s *PgvectorEmbeddingStore) Find(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	return cosineSearch(s.repo.DB(ctx), s.repo.Table(), options...)
}

// Exists checks if a snippet matching the options exists.
func (s *PgvectorEmbeddingStore) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	return s.repo.Exists(ctx, options...)
}

// SnippetIDs returns snippet IDs matching the given options.
func (s *PgvectorEmbeddingStore) SnippetIDs(ctx context.Context, options ...repository.Option) ([]string, error) {
	var found []string
	db := database.ApplyOptions(s.repo.DB(ctx), options...)
	err := db.Pluck("snippet_id", &found).Error
	if err != nil {
		return nil, err
	}
	return found, nil
}

// DeleteBy removes documents matching the given options.
func (s *PgvectorEmbeddingStore) DeleteBy(ctx context.Context, options ...repository.Option) error {
	return s.repo.DeleteBy(ctx, options...)
}
