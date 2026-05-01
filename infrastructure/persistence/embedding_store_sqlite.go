package persistence

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SQLiteEmbeddingStore implements search.Store for SQLite.
// Stores embeddings as JSON and performs cosine similarity search in-memory.
type SQLiteEmbeddingStore struct {
	database.Repository[search.Result, SQLiteEmbeddingModel]
	logger zerolog.Logger
}

// NewSQLiteEmbeddingStore creates a new SQLiteEmbeddingStore.
func NewSQLiteEmbeddingStore(db database.Database, taskName TaskName, logger zerolog.Logger) (*SQLiteEmbeddingStore, error) {
	tableName := fmt.Sprintf("kodit_%s_embeddings", taskName)
	s := &SQLiteEmbeddingStore{
		Repository: database.NewRepositoryForTable[search.Result, SQLiteEmbeddingModel](
			db, sqliteEmbeddingMapper{}, "embedding", tableName,
		),
		logger: logger,
	}

	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding JSON NOT NULL
)`, tableName)

	if err := db.Session(context.Background()).Exec(createTableSQL).Error; err != nil {
		return nil, fmt.Errorf("create table %s: %w", tableName, err)
	}

	return s, nil
}

// Index persists pre-computed embeddings using batched upsert.
// Documents without a vector are skipped (this store does not index text).
func (s *SQLiteEmbeddingStore) Index(ctx context.Context, docs []search.Document) error {
	models := make([]SQLiteEmbeddingModel, 0, len(docs))
	for _, doc := range docs {
		vec := doc.Vector()
		if doc.SnippetID() == "" || len(vec) == 0 {
			continue
		}
		models = append(models, SQLiteEmbeddingModel{
			SnippetID: doc.SnippetID(),
			Embedding: Float64Slice(vec),
		})
	}
	if len(models) == 0 {
		return nil
	}

	tableName := s.Table()
	db := s.DB(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		return tx.Table(tableName).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "snippet_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"embedding"}),
		}).CreateInBatches(models, saveAllBatchSize).Error
	})
}

// Find returns ranked vector-similarity results when WithEmbedding is supplied;
// otherwise delegates to the embedded Repository for plain snippet_id lookups.
//
// SQLite has no vector index, so similarity is computed in-memory across all
// rows matching the lookup filter.
func (s *SQLiteEmbeddingStore) Find(ctx context.Context, opts ...repository.Option) ([]search.Result, error) {
	q := repository.Build(opts...)
	queryEmbedding, ok := search.EmbeddingFrom(q)
	if !ok || len(queryEmbedding) == 0 {
		return s.Repository.Find(ctx, opts...)
	}

	limit := q.LimitValue()
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.loadRows(ctx, opts...)
	if err != nil {
		return nil, err
	}

	var allowed map[string]struct{}
	if ids := search.SnippetIDsFrom(q); len(ids) > 0 {
		allowed = make(map[string]struct{}, len(ids))
		for _, id := range ids {
			allowed[id] = struct{}{}
		}
	}

	return topKSimilar(queryEmbedding, rows, limit, allowed), nil
}

// loadRows loads embedding rows from the database, applying any search
// filters via JOINs.
func (s *SQLiteEmbeddingStore) loadRows(ctx context.Context, opts ...repository.Option) ([]vectorRow, error) {
	var entities []SQLiteEmbeddingModel

	q := repository.Build(opts...)
	db := database.ApplyConditions(s.DB(ctx), opts...)
	db = db.Table(s.Table())

	if filters, ok := search.FiltersFrom(q); ok {
		for _, opt := range filterJoinOptions(filters, "INTEGER") {
			db = database.ApplyOptions(db, opt)
		}
	}

	if err := db.Find(&entities).Error; err != nil {
		return nil, err
	}

	rows := make([]vectorRow, 0, len(entities))
	for _, e := range entities {
		if len(e.Embedding) == 0 {
			s.logger.Warn().Str("snippet_id", e.SnippetID).Msg("skipping empty embedding")
			continue
		}
		rows = append(rows, vectorRow{snippetID: e.SnippetID, embedding: e.Embedding})
	}

	return rows, nil
}
