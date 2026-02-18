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

// Search performs vector similarity search.
func (s *SQLiteVectorStore) Search(ctx context.Context, request search.Request) ([]search.Result, error) {
	if err := s.initialize(ctx); err != nil {
		return nil, err
	}

	query := request.Query()
	if query == "" {
		return []search.Result{}, nil
	}

	topK := request.TopK()
	if topK <= 0 {
		topK = 10
	}

	// Get embedding for query
	embResp, err := s.embedder.Embed(ctx, provider.NewEmbeddingRequest([]string{query}))
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	embeddings := embResp.Embeddings()
	if len(embeddings) == 0 {
		return []search.Result{}, nil
	}

	queryEmbedding := embeddings[0]

	// Load all embeddings from database
	vectors, err := s.loadVectors(ctx, request.SnippetIDs())
	if err != nil {
		return nil, err
	}

	if len(vectors) == 0 {
		return []search.Result{}, nil
	}

	// Compute similarities and find top-k
	var matches []SimilarityMatch
	snippetFilter := request.SnippetIDs()
	if len(snippetFilter) > 0 {
		filterSet := make(map[string]struct{}, len(snippetFilter))
		for _, id := range snippetFilter {
			filterSet[id] = struct{}{}
		}
		matches = TopKSimilarFiltered(queryEmbedding, vectors, topK, filterSet)
	} else {
		matches = TopKSimilar(queryEmbedding, vectors, topK)
	}

	// Convert to search results
	results := make([]search.Result, len(matches))
	for i, m := range matches {
		results[i] = search.NewResult(m.SnippetID(), m.Similarity())
	}

	return results, nil
}

// loadVectors loads embedding vectors from the database using GORM.
// If snippetIDs is provided, only loads those specific vectors.
func (s *SQLiteVectorStore) loadVectors(ctx context.Context, snippetIDs []string) ([]StoredVector, error) {
	var entities []SQLiteEmbeddingEntity

	q := s.repo.DB(ctx)
	if len(snippetIDs) > 0 {
		q = q.Where("snippet_id IN ?", snippetIDs)
	}
	if err := q.Find(&entities).Error; err != nil {
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

// HasEmbedding checks if a snippet has an embedding of the given type.
func (s *SQLiteVectorStore) HasEmbedding(ctx context.Context, snippetID string, embeddingType search.EmbeddingType) (bool, error) {
	if err := s.initialize(ctx); err != nil {
		return false, err
	}
	_ = embeddingType
	return s.repo.Exists(ctx, repository.WithCondition("snippet_id", snippetID))
}

// HasEmbeddings checks which snippet IDs have embeddings of the given type.
func (s *SQLiteVectorStore) HasEmbeddings(ctx context.Context, snippetIDs []string, embeddingType search.EmbeddingType) (map[string]bool, error) {
	if len(snippetIDs) == 0 {
		return map[string]bool{}, nil
	}

	if err := s.initialize(ctx); err != nil {
		return nil, err
	}
	_ = embeddingType

	var found []string
	err := s.repo.DB(ctx).Where("snippet_id IN ?", snippetIDs).Pluck("snippet_id", &found).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(found))
	for _, id := range found {
		result[id] = true
	}
	return result, nil
}

// Delete removes documents from the vector store.
func (s *SQLiteVectorStore) Delete(ctx context.Context, request search.DeleteRequest) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}
	ids := request.SnippetIDs()
	if len(ids) == 0 {
		return nil
	}
	return s.repo.DeleteBy(ctx, repository.WithConditionIn("snippet_id", ids))
}
