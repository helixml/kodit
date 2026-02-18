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

// ErrVectorInitializationFailed indicates VectorChord vector initialization failed.
var ErrVectorInitializationFailed = errors.New("failed to initialize VectorChord vector repository")

// ErrDimensionMismatch indicates embedding dimension doesn't match database.
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

// VectorChordEmbeddingStore implements search.EmbeddingStore using VectorChord PostgreSQL extension.
type VectorChordEmbeddingStore struct {
	repo   database.Repository[search.Embedding, PgEmbeddingModel]
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
		repo: database.NewRepositoryForTable[search.Embedding, PgEmbeddingModel](
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
// operator class (e.g. vector_l2_ops instead of vector_cosine_ops).
func (s *VectorChordEmbeddingStore) migrateIndex(ctx context.Context) error {
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

// SaveAll persists pre-computed embeddings using upsert.
func (s *VectorChordEmbeddingStore) SaveAll(ctx context.Context, embeddings []search.Embedding) error {
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
func (s *VectorChordEmbeddingStore) Find(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	return cosineSearch(s.repo.DB(ctx), s.repo.Table(), options...)
}

// Exists checks if a snippet matching the options exists.
func (s *VectorChordEmbeddingStore) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	return s.repo.Exists(ctx, options...)
}

// SnippetIDs returns snippet IDs matching the given options.
func (s *VectorChordEmbeddingStore) SnippetIDs(ctx context.Context, options ...repository.Option) ([]string, error) {
	var found []string
	db := database.ApplyOptions(s.repo.DB(ctx), options...)
	err := db.Pluck("snippet_id", &found).Error
	if err != nil {
		return nil, err
	}
	return found, nil
}

// DeleteBy removes documents matching the given options.
func (s *VectorChordEmbeddingStore) DeleteBy(ctx context.Context, options ...repository.Option) error {
	return s.repo.DeleteBy(ctx, options...)
}
