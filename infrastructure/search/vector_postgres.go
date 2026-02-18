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

// Find performs vector similarity search.
func (s *PgvectorStore) Find(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	if err := s.initialize(ctx); err != nil {
		return nil, err
	}
	return cosineSearch(ctx, s.repo.DB(ctx), s.repo.Table(), options...)
}

// Exists checks if a snippet matching the options exists.
func (s *PgvectorStore) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	if err := s.initialize(ctx); err != nil {
		return false, err
	}
	return s.repo.Exists(ctx, options...)
}

// SnippetIDs returns snippet IDs matching the given options.
func (s *PgvectorStore) SnippetIDs(ctx context.Context, options ...repository.Option) ([]string, error) {
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
func (s *PgvectorStore) DeleteBy(ctx context.Context, options ...repository.Option) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}
	return s.repo.DeleteBy(ctx, options...)
}
