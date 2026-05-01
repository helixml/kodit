package persistence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"

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

// VectorChordEmbeddingStore implements search.Store using the VectorChord
// PostgreSQL extension. Table creation is deferred to the first Index call,
// using the actual embedding dimension. Read/delete methods return empty
// results when the table does not yet exist.
type VectorChordEmbeddingStore struct {
	database.Repository[search.Result, PgEmbeddingModel]
	logger  zerolog.Logger
	indexMu sync.Mutex

	onRebuilt  func(context.Context)
	tableMu    sync.Mutex
	tableReady atomic.Bool
}

// NewVectorChordEmbeddingStore creates a new VectorChordEmbeddingStore.
// The VectorChord extension and table are created lazily on the first Index
// call using the actual embedding dimension. On construction, we probe
// pg_class to determine whether the table already exists so that read/delete
// methods can return early without hitting a missing-relation error.
//
// onRebuilt is called (at most once) if an existing table had to be dropped
// and recreated due to a dimension mismatch; pass nil if no action is needed.
func NewVectorChordEmbeddingStore(db database.Database, taskName TaskName, onRebuilt func(context.Context), logger zerolog.Logger) *VectorChordEmbeddingStore {
	tableName := fmt.Sprintf("vectorchord_%s_embeddings", taskName)
	s := &VectorChordEmbeddingStore{
		Repository: database.NewRepositoryForTable[search.Result, PgEmbeddingModel](
			db, pgEmbeddingMapper{}, "embedding", tableName,
		),
		onRebuilt: onRebuilt,
		logger:    logger,
	}

	var count int64
	s.DB(context.Background()).Raw(
		"SELECT count(*) FROM pg_class WHERE relname = ? AND relkind = 'r'", tableName,
	).Scan(&count)
	if count > 0 {
		s.tableReady.Store(true)
	} else {
		logger.Warn().Str("table", tableName).Msg("embedding table does not exist yet; read/delete operations will return empty until first Index creates it")
	}

	return s
}

// Find performs vector similarity search when WithEmbedding is supplied;
// otherwise delegates to the embedded Repository for plain snippet_id lookups.
//
// Returns nil if the table has not yet been created.
func (s *VectorChordEmbeddingStore) Find(ctx context.Context, opts ...repository.Option) ([]search.Result, error) {
	if !s.tableReady.Load() {
		return nil, nil
	}

	q := repository.Build(opts...)
	embedding, ok := search.EmbeddingFrom(q)
	if !ok || len(embedding) == 0 {
		return s.Repository.Find(ctx, opts...)
	}

	limit := q.LimitValue()
	if limit <= 0 {
		limit = 10
	}

	queryEmbedding := database.NewPgVector(embedding).String()
	augmented := []repository.Option{
		repository.WithSelect("snippet_id, embedding <=> ? AS score", queryEmbedding),
		repository.WithRawOrder("score ASC"),
		repository.WithLimit(limit),
	}
	if filters, ok := search.FiltersFrom(q); ok {
		augmented = append(augmented, filterJoinOptions(filters, "bigint")...)
	}
	if snippetIDs := search.SnippetIDsFrom(q); len(snippetIDs) > 0 {
		augmented = append(augmented, search.WithSnippetIDs(snippetIDs))
	}

	// vchordrq.probes must be set within a transaction so the SET LOCAL
	// is visible to the SELECT that follows.
	var count int64
	if err := s.DB(ctx).Table(s.Table()).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("count for probes: %w", err)
	}
	probes := probeCount(count)

	var results []search.Result
	err := s.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(fmt.Sprintf("SET LOCAL vchordrq.probes = %d", probes)).Error; err != nil {
			return fmt.Errorf("set vchordrq.probes: %w", err)
		}
		var entities []PgEmbeddingModel
		db := database.ApplyOptions(tx.Table(s.Table()), augmented...)
		if err := db.Scan(&entities).Error; err != nil {
			return err
		}
		results = make([]search.Result, len(entities))
		for i, e := range entities {
			results[i] = s.Mapper().ToDomain(e)
		}
		return nil
	})
	return results, err
}

// DeleteBy removes embeddings, silently succeeding if the table hasn't been created yet.
func (s *VectorChordEmbeddingStore) DeleteBy(ctx context.Context, opts ...repository.Option) error {
	if !s.tableReady.Load() {
		return nil
	}
	return s.Repository.DeleteBy(ctx, opts...)
}

// Exists checks for matching embeddings, returning false if the table hasn't been created yet.
func (s *VectorChordEmbeddingStore) Exists(ctx context.Context, opts ...repository.Option) (bool, error) {
	if !s.tableReady.Load() {
		return false, nil
	}
	return s.Repository.Exists(ctx, opts...)
}

// ensureTable creates the VectorChord extension and embedding table if they
// do not already exist. If the table exists with a different vector dimension
// it is dropped and recreated, and the onRebuilt callback fires.
//
// Called from Index only — dimension is derived from the documents themselves,
// so no probe call is needed.
func (s *VectorChordEmbeddingStore) ensureTable(ctx context.Context, dimension int) error {
	s.tableMu.Lock()
	defer s.tableMu.Unlock()
	if s.tableReady.Load() {
		return nil
	}

	tableName := s.Table()
	rawDB := s.DB(ctx)

	if err := rawDB.Exec(vcCreateVChordExtension).Error; err != nil {
		return errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create extension: %w", err))
	}

	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding VECTOR(%d) NOT NULL
)`, tableName, dimension)

	if err := rawDB.Exec(createTableSQL).Error; err != nil {
		return errors.Join(ErrVectorInitializationFailed, fmt.Errorf("create table: %w", err))
	}

	var dbDimension int
	checkDimensionSQL := fmt.Sprintf(vcCheckDimensionTemplate, tableName)
	result := rawDB.Raw(checkDimensionSQL).Scan(&dbDimension)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return errors.Join(ErrVectorInitializationFailed, fmt.Errorf("check dimension: %w", result.Error))
	}

	if result.RowsAffected > 0 && dbDimension != dimension {
		s.logger.Warn().Str("table", tableName).Int("old_dimension", dbDimension).Int("new_dimension", dimension).Msg("embedding dimension changed, dropping old table for re-indexing")

		dropSQL := fmt.Sprintf("DROP TABLE %s CASCADE", tableName)
		if err := rawDB.Exec(dropSQL).Error; err != nil {
			return errors.Join(ErrVectorInitializationFailed, fmt.Errorf("drop table: %w", err))
		}
		if err := rawDB.Exec(createTableSQL).Error; err != nil {
			return errors.Join(ErrVectorInitializationFailed, fmt.Errorf("recreate table: %w", err))
		}

		// Force pooled connections to close so subsequent queries use fresh
		// connections that see the new table, not stale cached state.
		if sqlDB, dbErr := rawDB.DB(); dbErr == nil {
			sqlDB.SetMaxIdleConns(0)
			sqlDB.SetMaxIdleConns(10)
		}

		if s.onRebuilt != nil {
			s.onRebuilt(ctx)
		}
	}

	s.tableReady.Store(true)
	return nil
}

// Index persists pre-computed vectors using batched upsert, then ensures
// the vchordrq index exists (it requires data for K-means clustering).
// Documents without a vector are skipped (this store does not index text).
func (s *VectorChordEmbeddingStore) Index(ctx context.Context, docs []search.Document) error {
	models := make([]PgEmbeddingModel, 0, len(docs))
	var dimension int
	for _, doc := range docs {
		vec := doc.Vector()
		if doc.SnippetID() == "" || len(vec) == 0 {
			continue
		}
		if dimension == 0 {
			dimension = len(vec)
		}
		models = append(models, PgEmbeddingModel{
			SnippetID: doc.SnippetID(),
			Embedding: database.NewPgVector(vec),
		})
	}
	if len(models) == 0 {
		return nil
	}
	if err := s.ensureTable(ctx, dimension); err != nil {
		return err
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
		return nil
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

	s.logger.Info().Str("table", tableName).Int64("rows", count).Int64("lists", lists).Msg("creating vchordrq index")

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
