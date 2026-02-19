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

// SaveAll persists pre-computed embeddings using upsert, then ensures
// the vchordrq index exists (it requires data for K-means clustering).
func (s *VectorChordEmbeddingStore) SaveAll(ctx context.Context, embeddings []search.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	tableName := s.Table()
	db := s.DB(ctx)

	err := db.Transaction(func(tx *gorm.DB) error {
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
	if err != nil {
		return err
	}

	return s.ensureIndex(ctx)
}

// ensureIndex creates the vchordrq index if it doesn't already exist.
// Must be called after data has been inserted so K-means clustering has
// vectors to work with.
func (s *VectorChordEmbeddingStore) ensureIndex(ctx context.Context) error {
	tableName := s.Table()
	db := s.DB(ctx)

	var method string
	query := fmt.Sprintf(vcCheckIndexMethodTemplate, tableName)
	result := db.Raw(query).Scan(&method)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check index method: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return nil // index already exists
	}

	var count int64
	if err := db.Table(tableName).Count(&count).Error; err != nil {
		return fmt.Errorf("count rows: %w", err)
	}

	lists := max(count/10, 1)

	indexSQL := fmt.Sprintf(`
CREATE INDEX IF NOT EXISTS %s_idx
ON %s
USING vchordrq (embedding vector_cosine_ops) WITH (options = $$
residual_quantization = true
[build.internal]
lists = [%d]
$$)`, tableName, tableName, lists)

	s.logger.Info("creating vchordrq index",
		slog.String("table", tableName),
		slog.Int64("rows", count),
		slog.Int64("lists", lists),
	)

	if err := db.Exec(indexSQL).Error; err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	return nil
}

// Search performs vector similarity search within a transaction so that
// the vchordrq.probes session variable is visible to the query.
func (s *VectorChordEmbeddingStore) Search(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	db := s.DB(ctx)

	var results []search.Result
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL vchordrq.probes = 10").Error; err != nil {
			return fmt.Errorf("set vchordrq.probes: %w", err)
		}
		var searchErr error
		results, searchErr = cosineSearch(tx, s.Table(), options...)
		return searchErr
	})
	return results, err
}
