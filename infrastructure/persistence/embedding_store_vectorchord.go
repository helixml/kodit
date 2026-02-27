package persistence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"

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

// VectorChordEmbeddingStore implements search.EmbeddingStore using VectorChord PostgreSQL extension.
type VectorChordEmbeddingStore struct {
	database.Repository[search.Embedding, PgEmbeddingModel]
	logger  *slog.Logger
	indexMu sync.Mutex
}

// NewVectorChordEmbeddingStore creates a new VectorChordEmbeddingStore, eagerly
// initializing the extension, table, index, and verifying the dimension.
// The returned bool is true when the table was dropped and recreated due to a
// dimension mismatch (e.g. the user switched embedding providers).
func NewVectorChordEmbeddingStore(ctx context.Context, db database.Database, taskName TaskName, dimension int, logger *slog.Logger) (*VectorChordEmbeddingStore, bool, error) {
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
		return nil, false, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create extension: %w", err))
	}

	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding VECTOR(%d) NOT NULL
)`, tableName, dimension)

	// Create table (dynamic dimension requires raw SQL)
	if err := rawDB.Exec(createTableSQL).Error; err != nil {
		return nil, false, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create table: %w", err))
	}

	// Check whether the existing table dimension matches the provider.
	var dbDimension int
	checkDimensionSQL := fmt.Sprintf(vcCheckDimensionTemplate, tableName)
	result := rawDB.Raw(checkDimensionSQL).Scan(&dbDimension)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, false, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("check dimension: %w", result.Error))
	}

	rebuilt := false
	if result.RowsAffected > 0 && dbDimension != dimension {
		logger.Warn("embedding dimension changed, dropping old table for re-indexing",
			slog.String("table", tableName),
			slog.Int("old_dimension", dbDimension),
			slog.Int("new_dimension", dimension),
		)

		dropSQL := fmt.Sprintf("DROP TABLE %s CASCADE", tableName)
		if err := rawDB.Exec(dropSQL).Error; err != nil {
			return nil, false, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("drop table: %w", err))
		}
		if err := rawDB.Exec(createTableSQL).Error; err != nil {
			return nil, false, errors.Join(ErrVectorInitializationFailed, fmt.Errorf("recreate table: %w", err))
		}
		rebuilt = true
	}

	return s, rebuilt, nil
}

// SaveAll persists pre-computed embeddings using batched upsert, then ensures
// the vchordrq index exists (it requires data for K-means clustering).
func (s *VectorChordEmbeddingStore) SaveAll(ctx context.Context, embeddings []search.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	models := make([]PgEmbeddingModel, len(embeddings))
	for i, emb := range embeddings {
		models[i] = PgEmbeddingModel{
			SnippetID: emb.SnippetID(),
			Embedding: database.NewPgVector(emb.Vector()),
		}
	}

	tableName := s.Table()
	db := s.DB(ctx)

	err := db.Transaction(func(tx *gorm.DB) error {
		return tx.Table(tableName).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "snippet_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"embedding"}),
		}).CreateInBatches(models, saveAllBatchSize).Error
	})
	if err != nil {
		return err
	}

	return s.ensureIndex(ctx)
}

// ensureIndex creates the vchordrq index if it doesn't already exist.
// Must be called after data has been inserted so K-means clustering has
// vectors to work with. A mutex serializes callers within this process;
// the constraint-violation check handles races across separate processes.
func (s *VectorChordEmbeddingStore) ensureIndex(ctx context.Context) error {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()

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
		// Another process may have created the index concurrently,
		// producing a unique_violation (SQLSTATE 23505) on pg_class_relname_nsp_index.
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return nil
		}
		return fmt.Errorf("create index: %w", err)
	}
	return nil
}

// probeCount returns the number of IVF probes for a given row count.
// The index is built with lists = max(count/10, 1), so probes scales
// as sqrt(lists) with a floor of 10.
func probeCount(rows int64) int {
	lists := max(rows/10, 1)
	return max(int(math.Sqrt(float64(lists))), 10)
}

// Search performs vector similarity search within a transaction so that
// the vchordrq.probes session variable is visible to the query.
func (s *VectorChordEmbeddingStore) Search(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	var count int64
	db := s.DB(ctx)
	if err := db.Table(s.Table()).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("count for probes: %w", err)
	}
	probes := probeCount(count)

	var results []search.Result
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(fmt.Sprintf("SET LOCAL vchordrq.probes = %d", probes)).Error; err != nil {
			return fmt.Errorf("set vchordrq.probes: %w", err)
		}
		var searchErr error
		results, searchErr = cosineSearch(tx, s.Table(), options...)
		return searchErr
	})
	return results, err
}
