package search

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/database"
)

// Float64Slice is a custom type for JSON serialization of []float64 in SQLite.
type Float64Slice []float64

// Scan implements sql.Scanner for reading JSON from SQLite.
func (f *Float64Slice) Scan(value any) error {
	if value == nil {
		*f = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into Float64Slice", value)
	}

	return json.Unmarshal(data, f)
}

// Value implements driver.Valuer for writing JSON to SQLite.
func (f Float64Slice) Value() (driver.Value, error) {
	if f == nil {
		return nil, nil
	}
	return json.Marshal(f)
}

// SQLiteEmbeddingEntity represents a vector embedding in SQLite.
// Table routing is done via .Table(name) at the call site because GORM
// caches schemas by type and dynamic TableName() does not work across
// multiple table names for the same struct type.
type SQLiteEmbeddingEntity struct {
	ID        int64        `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string       `gorm:"column:snippet_id;uniqueIndex"`
	Embedding Float64Slice `gorm:"column:embedding;type:json"`
}

// newSQLiteEmbeddingEntity creates a SQLiteEmbeddingEntity ready for insertion.
func newSQLiteEmbeddingEntity(snippetID string, embedding []float64) SQLiteEmbeddingEntity {
	cp := make(Float64Slice, len(embedding))
	copy(cp, embedding)
	return SQLiteEmbeddingEntity{
		SnippetID: snippetID,
		Embedding: cp,
	}
}

// ErrSQLiteVectorInitializationFailed indicates SQLite vector initialization failed.
var ErrSQLiteVectorInitializationFailed = errors.New("failed to initialize SQLite vector store")

// SQLiteVectorStore implements search.VectorStore for SQLite.
// Stores embeddings as JSON and performs cosine similarity search in-memory.
type SQLiteVectorStore struct {
	repo        database.Repository[SQLiteEmbeddingEntity, SQLiteEmbeddingEntity]
	embedder    provider.Embedder
	logger      *slog.Logger
	initialized bool
	mu          sync.Mutex
}

// NewSQLiteVectorStore creates a new SQLiteVectorStore.
func NewSQLiteVectorStore(db database.Database, taskName TaskName, embedder provider.Embedder, logger *slog.Logger) *SQLiteVectorStore {
	if logger == nil {
		logger = slog.Default()
	}
	tableName := fmt.Sprintf("kodit_%s_embeddings", taskName)
	return &SQLiteVectorStore{
		repo:     newSQLiteEmbeddingRepository(db, tableName),
		embedder: embedder,
		logger:   logger,
	}
}

func (s *SQLiteVectorStore) initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	if err := s.createTable(ctx); err != nil {
		return errors.Join(ErrSQLiteVectorInitializationFailed, err)
	}

	s.initialized = true
	return nil
}

func (s *SQLiteVectorStore) createTable(ctx context.Context) error {
	tableName := s.repo.Table()
	// Raw SQL because GORM's AutoMigrate caches schemas by type, which
	// conflicts with our dynamic table names.
	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding JSON NOT NULL
)`, tableName)

	return s.repo.DB(ctx).Exec(createTableSQL).Error
}

// Index adds documents to the vector store with embeddings.
func (s *SQLiteVectorStore) Index(ctx context.Context, request search.IndexRequest) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}
	return indexDocuments(ctx, &s.repo, s.embedder, s.logger, request, sqliteEntityFactory)
}

// Find performs vector similarity search using pre-computed embedding from options.
func (s *SQLiteVectorStore) Find(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	if err := s.initialize(ctx); err != nil {
		return nil, err
	}

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
// Condition options (e.g. snippet_id IN) are applied as WHERE filters.
func (s *SQLiteVectorStore) loadVectors(ctx context.Context, options ...repository.Option) ([]StoredVector, error) {
	var entities []SQLiteEmbeddingEntity

	q := repository.Build(options...)
	db := database.ApplyConditions(s.repo.DB(ctx), options...)

	if filters, ok := search.FiltersFrom(q); ok {
		db = applySearchFilters(db, filters)
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
func (s *SQLiteVectorStore) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	if err := s.initialize(ctx); err != nil {
		return false, err
	}
	return s.repo.Exists(ctx, options...)
}

// SnippetIDs returns snippet IDs matching the given options.
func (s *SQLiteVectorStore) SnippetIDs(ctx context.Context, options ...repository.Option) ([]string, error) {
	if err := s.initialize(ctx); err != nil {
		return nil, err
	}
	var found []string
	db := database.ApplyOptions(s.repo.DB(ctx), options...)
	err := db.Pluck("snippet_id", &found).Error
	if err != nil {
		return nil, err
	}
	return found, nil
}

// DeleteBy removes documents matching the given options.
func (s *SQLiteVectorStore) DeleteBy(ctx context.Context, options ...repository.Option) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}
	return s.repo.DeleteBy(ctx, options...)
}
