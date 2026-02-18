package persistence

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SQLiteEmbeddingStore implements search.EmbeddingStore for SQLite.
// Stores embeddings as JSON and performs cosine similarity search in-memory.
type SQLiteEmbeddingStore struct {
	repo   database.Repository[search.Embedding, SQLiteEmbeddingModel]
	logger *slog.Logger
}

// NewSQLiteEmbeddingStore creates a new SQLiteEmbeddingStore.
func NewSQLiteEmbeddingStore(db database.Database, taskName TaskName, logger *slog.Logger) (*SQLiteEmbeddingStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	tableName := fmt.Sprintf("kodit_%s_embeddings", taskName)
	s := &SQLiteEmbeddingStore{
		repo: database.NewRepositoryForTable[search.Embedding, SQLiteEmbeddingModel](
			db, sqliteEmbeddingMapper{}, "embedding", tableName,
		),
		logger: logger,
	}

	// Create table eagerly
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

// SaveAll persists pre-computed embeddings using upsert.
func (s *SQLiteEmbeddingStore) SaveAll(ctx context.Context, embeddings []search.Embedding) error {
	if len(embeddings) == 0 {
		return nil
	}

	tableName := s.repo.Table()
	db := s.repo.DB(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		for _, emb := range embeddings {
			model := SQLiteEmbeddingModel{
				SnippetID: emb.SnippetID(),
				Embedding: Float64Slice(emb.Vector()),
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

// Find performs vector similarity search using pre-computed embedding from options.
func (s *SQLiteEmbeddingStore) Find(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	q := repository.Build(options...)
	queryEmbedding, ok := search.EmbeddingFrom(q)
	if !ok || len(queryEmbedding) == 0 {
		return []search.Result{}, nil
	}

	limit := q.LimitValue()
	if limit <= 0 {
		limit = 10
	}

	// Load all embeddings from database, applying condition filters
	vectors, err := s.loadVectors(ctx, options...)
	if err != nil {
		return nil, err
	}

	if len(vectors) == 0 {
		return []search.Result{}, nil
	}

	// Compute similarities and find top-k
	snippetIDs := search.SnippetIDsFrom(q)
	var matches []SimilarityMatch
	if len(snippetIDs) > 0 {
		filterSet := make(map[string]struct{}, len(snippetIDs))
		for _, id := range snippetIDs {
			filterSet[id] = struct{}{}
		}
		matches = TopKSimilarFiltered(queryEmbedding, vectors, limit, filterSet)
	} else {
		matches = TopKSimilar(queryEmbedding, vectors, limit)
	}

	// Convert to search results
	results := make([]search.Result, len(matches))
	for i, m := range matches {
		results[i] = search.NewResult(m.SnippetID(), m.Similarity())
	}

	return results, nil
}

// loadVectors loads embedding vectors from the database using GORM.
func (s *SQLiteEmbeddingStore) loadVectors(ctx context.Context, options ...repository.Option) ([]StoredVector, error) {
	var entities []SQLiteEmbeddingModel

	q := repository.Build(options...)
	db := database.ApplyConditions(s.repo.DB(ctx), options...)

	if filters, ok := search.FiltersFrom(q); ok {
		db = database.ApplySearchFilters(db, filters)
	}

	if err := db.Find(&entities).Error; err != nil {
		return nil, err
	}

	vectors := make([]StoredVector, 0, len(entities))
	for _, e := range entities {
		if len(e.Embedding) == 0 {
			s.logger.Warn("skipping empty embedding", "snippet_id", e.SnippetID)
			continue
		}
		vectors = append(vectors, NewStoredVector(e.SnippetID, e.Embedding))
	}

	return vectors, nil
}

// Exists checks if a snippet matching the options exists.
func (s *SQLiteEmbeddingStore) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	return s.repo.Exists(ctx, options...)
}

// SnippetIDs returns snippet IDs matching the given options.
func (s *SQLiteEmbeddingStore) SnippetIDs(ctx context.Context, options ...repository.Option) ([]string, error) {
	var found []string
	db := database.ApplyOptions(s.repo.DB(ctx), options...)
	err := db.Pluck("snippet_id", &found).Error
	if err != nil {
		return nil, err
	}
	return found, nil
}

// DeleteBy removes documents matching the given options.
func (s *SQLiteEmbeddingStore) DeleteBy(ctx context.Context, options ...repository.Option) error {
	return s.repo.DeleteBy(ctx, options...)
}
