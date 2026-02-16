package search

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/provider"
	"gorm.io/gorm"
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
type SQLiteEmbeddingEntity struct {
	ID        int64        `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string       `gorm:"column:snippet_id;uniqueIndex"`
	Embedding Float64Slice `gorm:"column:embedding;type:json"`
}

// TableName returns the table name for this entity.
func (SQLiteEmbeddingEntity) TableName() string {
	return "sqlite_embeddings"
}

// ErrSQLiteVectorInitializationFailed indicates SQLite vector initialization failed.
var ErrSQLiteVectorInitializationFailed = errors.New("failed to initialize SQLite vector store")

// SQLiteVectorStore implements search.VectorStore for SQLite.
// Stores embeddings as JSON and performs cosine similarity search in-memory.
type SQLiteVectorStore struct {
	db          *gorm.DB
	embedder    provider.Embedder
	logger      *slog.Logger
	tableName   string
	initialized bool
	mu          sync.Mutex
}

// NewSQLiteVectorStore creates a new SQLiteVectorStore.
func NewSQLiteVectorStore(db *gorm.DB, taskName TaskName, embedder provider.Embedder, logger *slog.Logger) *SQLiteVectorStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &SQLiteVectorStore{
		db:        db,
		embedder:  embedder,
		logger:    logger,
		tableName: fmt.Sprintf("kodit_%s_embeddings", taskName),
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
	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding JSON NOT NULL
)`, s.tableName)

	return s.db.WithContext(ctx).Exec(createTableSQL).Error
}

func (s *SQLiteVectorStore) existingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var existingIDs []string
	query := fmt.Sprintf("SELECT snippet_id FROM %s WHERE snippet_id IN ?", s.tableName)
	err := s.db.WithContext(ctx).Raw(query, ids).Scan(&existingIDs).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		result[id] = struct{}{}
	}
	return result, nil
}

// Index adds documents to the vector store with embeddings.
func (s *SQLiteVectorStore) Index(ctx context.Context, request search.IndexRequest) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}

	documents := request.Documents()
	if len(documents) == 0 {
		return nil
	}

	// Filter out already indexed documents
	ids := make([]string, len(documents))
	for i, doc := range documents {
		ids[i] = doc.SnippetID()
	}

	existing, err := s.existingIDs(ctx, ids)
	if err != nil {
		return err
	}

	var toIndex []search.Document
	for _, doc := range documents {
		if _, exists := existing[doc.SnippetID()]; !exists {
			toIndex = append(toIndex, doc)
		}
	}

	if len(toIndex) == 0 {
		s.logger.Info("no new documents to index")
		return nil
	}

	// Get embeddings for documents
	texts := make([]string, len(toIndex))
	for i, doc := range toIndex {
		texts[i] = doc.Text()
	}

	embResp, err := s.embedder.Embed(ctx, provider.NewEmbeddingRequest(texts))
	if err != nil {
		return fmt.Errorf("generate embeddings: %w", err)
	}

	embeddings := embResp.Embeddings()
	if len(embeddings) != len(toIndex) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(toIndex))
	}

	// Insert documents with embeddings
	insertQuery := fmt.Sprintf(`
INSERT INTO %s (snippet_id, embedding)
VALUES (?, ?)
ON CONFLICT (snippet_id) DO UPDATE
SET embedding = EXCLUDED.embedding`, s.tableName)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, doc := range toIndex {
			embeddingJSON, err := json.Marshal(embeddings[i])
			if err != nil {
				return fmt.Errorf("marshal embedding: %w", err)
			}
			if err := tx.Exec(insertQuery, doc.SnippetID(), string(embeddingJSON)).Error; err != nil {
				return err
			}
		}
		return nil
	})
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

// loadVectors loads embedding vectors from the database.
// If snippetIDs is provided, only loads those specific vectors.
func (s *SQLiteVectorStore) loadVectors(ctx context.Context, snippetIDs []string) ([]StoredVector, error) {
	var rows []struct {
		SnippetID string `gorm:"column:snippet_id"`
		Embedding string `gorm:"column:embedding"`
	}

	var err error
	if len(snippetIDs) > 0 {
		query := fmt.Sprintf("SELECT snippet_id, embedding FROM %s WHERE snippet_id IN ?", s.tableName)
		err = s.db.WithContext(ctx).Raw(query, snippetIDs).Scan(&rows).Error
	} else {
		query := fmt.Sprintf("SELECT snippet_id, embedding FROM %s", s.tableName)
		err = s.db.WithContext(ctx).Raw(query).Scan(&rows).Error
	}

	if err != nil {
		return nil, err
	}

	vectors := make([]StoredVector, 0, len(rows))
	for _, row := range rows {
		var embedding []float64
		if err := json.Unmarshal([]byte(row.Embedding), &embedding); err != nil {
			s.logger.Warn("failed to parse embedding", "snippet_id", row.SnippetID, "error", err)
			continue
		}
		vectors = append(vectors, NewStoredVector(row.SnippetID, embedding))
	}

	return vectors, nil
}

// HasEmbedding checks if a snippet has an embedding of the given type.
func (s *SQLiteVectorStore) HasEmbedding(ctx context.Context, snippetID string, embeddingType search.EmbeddingType) (bool, error) {
	if err := s.initialize(ctx); err != nil {
		return false, err
	}

	// Note: embeddingType is not used here because SQLite uses separate tables per task
	_ = embeddingType

	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE snippet_id = ?", s.tableName)
	err := s.db.WithContext(ctx).Raw(query, snippetID).Scan(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// HasEmbeddings checks which snippet IDs have embeddings of the given type.
func (s *SQLiteVectorStore) HasEmbeddings(ctx context.Context, snippetIDs []string, embeddingType search.EmbeddingType) (map[string]bool, error) {
	if len(snippetIDs) == 0 {
		return map[string]bool{}, nil
	}

	if err := s.initialize(ctx); err != nil {
		return nil, err
	}

	// Note: embeddingType is not used here because SQLite uses separate tables per task
	_ = embeddingType

	var found []string
	query := fmt.Sprintf("SELECT snippet_id FROM %s WHERE snippet_id IN ?", s.tableName)
	err := s.db.WithContext(ctx).Raw(query, snippetIDs).Scan(&found).Error
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

	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE snippet_id IN ?", s.tableName)
	return s.db.WithContext(ctx).Exec(deleteSQL, ids).Error
}
