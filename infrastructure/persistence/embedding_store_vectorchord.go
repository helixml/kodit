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

// SQL queries that must stay as raw SQL (extensions, indexes, catalog).
const (
	vcCreateVChordExtension = `CREATE EXTENSION IF NOT EXISTS vchord CASCADE`

	vcCreateHNSWIndexTemplate = `
CREATE INDEX IF NOT EXISTS %s_idx
ON %s
USING vchordrq (embedding vector_cosine_ops)`

	vcCheckDimensionTemplate = `
SELECT a.atttypmod as dimension
FROM pg_attribute a
JOIN pg_class c ON a.attrelid = c.oid
WHERE c.relname = '%s'
AND a.attname = 'embedding'`

	vcCheckIndexMethodTemplate = `
SELECT amname FROM pg_index i
JOIN pg_class c ON c.oid = i.indexrelid
JOIN pg_am a ON a.oid = c.relam
WHERE c.relname = '%s_idx'`
)

// ErrVectorInitializationFailed indicates VectorChord vector initialization failed.
var ErrVectorInitializationFailed = errors.New("failed to initialize VectorChord vector repository")

// ErrDimensionMismatch indicates embedding dimension doesn't match database.
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

// VectorChordEmbeddingStore implements search.EmbeddingStore using VectorChord PostgreSQL extension.
type VectorChordEmbeddingStore struct {
	database.Repository[search.Embedding, PgEmbeddingModel]
	logger *slog.Logger
}

// NewVectorChordEmbeddingStore creates a new VectorChordEmbeddingStore, eagerly
// initializing the extension, table, index, and verifying the dimension.
func NewVectorChordEmbeddingStore(ctx context.Context, db database.Database, taskName TaskName, dimension int, logger *slog.Logger) (*VectorChordEmbeddingStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	tableName := fmt.Sprintf("vectorchord_%s_embeddings", taskName)
	s := &VectorChordEmbeddingStore{
		Repository: database.NewRepositoryForTable[search.Embedding, PgEmbeddingModel](
			db, pgEmbeddingMapper{}, "embedding", tableName,
		),
		logger: logger,
	}

	rawDB := db.Session(ctx)

	// Create extension
	if err := rawDB.Exec(vcCreateVChordExtension).Error; err != nil {
		return nil, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create extension: %w", err))
	}

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
	indexSQL := fmt.Sprintf(vcCreateHNSWIndexTemplate, tableName, tableName)
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

// migrateIndex drops an existing index if it uses the wrong access method.
func (s *VectorChordEmbeddingStore) migrateIndex(ctx context.Context) error {
	tableName := s.Table()
	db := s.DB(ctx)

	var method string
	query := fmt.Sprintf(vcCheckIndexMethodTemplate, tableName)
	result := db.Raw(query).Scan(&method)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check index method: %w", result.Error)
	}
	if result.RowsAffected == 0 || method == "hnsw" {
		return nil
	}

	s.logger.Warn("dropping embedding index with wrong access method",
		slog.String("table", tableName),
		slog.String("old_method", method),
	)

	dropSQL := fmt.Sprintf("DROP INDEX IF EXISTS %s_idx", tableName)
	if err := db.Exec(dropSQL).Error; err != nil {
		return fmt.Errorf("drop old index: %w", err)
	}
	return nil
}

// SaveAll persists pre-computed embeddings using upsert.
func (s *VectorChordEmbeddingStore) SaveAll(ctx context.Context, embeddings []search.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	tableName := s.Table()
	db := s.DB(ctx)

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

// Search performs vector similarity search.
func (s *VectorChordEmbeddingStore) Search(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	return cosineSearch(s.DB(ctx), s.Table(), options...)
}
