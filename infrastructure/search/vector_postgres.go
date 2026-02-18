package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

// SQL queries specific to pgvector (extensions, indexes, catalog).
const (
	pgvCreateExtension = `CREATE EXTENSION IF NOT EXISTS vector`

	pgvCreateIndexTemplate = `
CREATE INDEX IF NOT EXISTS %s_idx
ON %s
USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100)`

	pgvCheckDimensionTemplate = `
SELECT a.atttypmod as dimension
FROM pg_attribute a
JOIN pg_class c ON a.attrelid = c.oid
WHERE c.relname = '%s'
AND a.attname = 'embedding'`
)

// ErrPgvectorInitializationFailed indicates pgvector initialization failed.
var ErrPgvectorInitializationFailed = errors.New("failed to initialize pgvector store")

// PgvectorStore implements search.VectorStore using PostgreSQL pgvector extension.
type PgvectorStore struct {
	repo        database.Repository[PgEmbeddingEntity, PgEmbeddingEntity]
	embedder    provider.Embedder
	logger      *slog.Logger
	initialized bool
	mu          sync.Mutex
}

// NewPgvectorStore creates a new PgvectorStore.
func NewPgvectorStore(db database.Database, taskName TaskName, embedder provider.Embedder, logger *slog.Logger) *PgvectorStore {
	if logger == nil {
		logger = slog.Default()
	}
	tableName := fmt.Sprintf("pgvector_%s_embeddings", taskName)
	return &PgvectorStore{
		repo:     newPgEmbeddingRepository(db, tableName),
		embedder: embedder,
		logger:   logger,
	}
}

func (s *PgvectorStore) initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	if err := s.createExtension(ctx); err != nil {
		return errors.Join(ErrPgvectorInitializationFailed, err)
	}

	if err := s.createTable(ctx); err != nil {
		return errors.Join(ErrPgvectorInitializationFailed, err)
	}

	s.initialized = true
	return nil
}

func (s *PgvectorStore) createExtension(ctx context.Context) error {
	return s.repo.DB(ctx).Exec(pgvCreateExtension).Error
}

func (s *PgvectorStore) createTable(ctx context.Context) error {
	tableName := s.repo.Table()
	db := s.repo.DB(ctx)

	// Get embedding dimension from provider
	resp, err := s.embedder.Embed(ctx, provider.NewEmbeddingRequest([]string{"dimension probe"}))
	if err != nil {
		return fmt.Errorf("probe embedding dimension: %w", err)
	}

	embeddings := resp.Embeddings()
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return errors.New("failed to obtain embedding dimension from provider")
	}

	dimension := len(embeddings[0])

	// Create table with correct vector dimension (dynamic dimension requires raw SQL)
	createTableSQL := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    embedding VECTOR(%d) NOT NULL
)`, tableName, dimension)
	if err := db.Exec(createTableSQL).Error; err != nil {
		return err
	}

	// Create index (ignore errors if index already exists with different parameters)
	indexSQL := fmt.Sprintf(pgvCreateIndexTemplate, tableName, tableName)
	if err := db.Exec(indexSQL).Error; err != nil {
		s.logger.Warn("failed to create index (may already exist)", "error", err)
	}

	// Verify dimension matches
	var dbDimension int
	checkDimensionSQL := fmt.Sprintf(pgvCheckDimensionTemplate, tableName)
	result := db.Raw(checkDimensionSQL).Scan(&dbDimension)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	if result.RowsAffected > 0 && dbDimension != dimension {
		return fmt.Errorf("%w: database has %d, provider has %d", ErrDimensionMismatch, dbDimension, dimension)
	}

	return nil
}

// Index adds documents to the vector index with embeddings.
func (s *PgvectorStore) Index(ctx context.Context, request search.IndexRequest) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}
	return indexDocuments(ctx, &s.repo, s.embedder, s.logger, request, pgEntityFactory)
}

// Search performs vector similarity search.
func (s *PgvectorStore) Search(ctx context.Context, request search.Request) ([]search.Result, error) {
	if err := s.initialize(ctx); err != nil {
		return nil, err
	}
	return cosineSearch(ctx, s.repo.DB(ctx), s.repo.Table(), s.embedder, request)
}

// HasEmbedding checks if a snippet has an embedding of the given type.
func (s *PgvectorStore) HasEmbedding(ctx context.Context, snippetID string, embeddingType search.EmbeddingType) (bool, error) {
	if err := s.initialize(ctx); err != nil {
		return false, err
	}
	_ = embeddingType
	return s.repo.Exists(ctx, repository.WithCondition("snippet_id", snippetID))
}

// HasEmbeddings checks which snippet IDs have embeddings of the given type.
func (s *PgvectorStore) HasEmbeddings(ctx context.Context, snippetIDs []string, embeddingType search.EmbeddingType) (map[string]bool, error) {
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

// Delete removes documents from the vector index.
func (s *PgvectorStore) Delete(ctx context.Context, request search.DeleteRequest) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}
	ids := request.SnippetIDs()
	if len(ids) == 0 {
		return nil
	}
	return s.repo.DeleteBy(ctx, repository.WithConditionIn("snippet_id", ids))
}
