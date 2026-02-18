package search

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"gorm.io/gorm"
)

// SQL statements for PostgreSQL native full-text search operations.
const (
	pgCreateBM25Table = `
CREATE TABLE IF NOT EXISTS postgres_bm25_documents (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL UNIQUE,
    passage TEXT NOT NULL,
    tsv TSVECTOR
)`

	pgCreateTSVIndex = `
CREATE INDEX IF NOT EXISTS postgres_bm25_documents_tsv_idx
ON postgres_bm25_documents
USING GIN(tsv)`

	pgCreateTriggerFunction = `
CREATE OR REPLACE FUNCTION postgres_bm25_update_tsv()
RETURNS trigger AS $$
BEGIN
    NEW.tsv := to_tsvector('english', NEW.passage);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql`

	pgCreateTrigger = `
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'postgres_bm25_tsv_trigger'
    ) THEN
        CREATE TRIGGER postgres_bm25_tsv_trigger
        BEFORE INSERT OR UPDATE ON postgres_bm25_documents
        FOR EACH ROW EXECUTE FUNCTION postgres_bm25_update_tsv();
    END IF;
END;
$$`

	pgInsertQuery = `
INSERT INTO postgres_bm25_documents (snippet_id, passage)
VALUES (?, ?)
ON CONFLICT (snippet_id) DO UPDATE
SET passage = EXCLUDED.passage`

	pgDeleteQuery = `DELETE FROM postgres_bm25_documents WHERE snippet_id IN ?`

	pgCheckExistingIDsQuery = `SELECT snippet_id FROM postgres_bm25_documents WHERE snippet_id IN ?`
)

// ErrPostgresBM25InitializationFailed indicates PostgreSQL FTS initialization failed.
var ErrPostgresBM25InitializationFailed = errors.New("failed to initialize PostgreSQL FTS BM25 store")

// PostgresBM25Store implements search.BM25Store using PostgreSQL native full-text search.
type PostgresBM25Store struct {
	db          *gorm.DB
	logger      *slog.Logger
	initialized bool
	mu          sync.Mutex
}

// NewPostgresBM25Store creates a new PostgresBM25Store.
func NewPostgresBM25Store(db *gorm.DB, logger *slog.Logger) *PostgresBM25Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &PostgresBM25Store{
		db:     db,
		logger: logger,
	}
}

func (s *PostgresBM25Store) initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	if err := s.createTable(ctx); err != nil {
		return errors.Join(ErrPostgresBM25InitializationFailed, err)
	}

	if err := s.createTrigger(ctx); err != nil {
		return errors.Join(ErrPostgresBM25InitializationFailed, err)
	}

	s.initialized = true
	return nil
}

func (s *PostgresBM25Store) createTable(ctx context.Context) error {
	db := s.db.WithContext(ctx)

	if err := db.Exec(pgCreateBM25Table).Error; err != nil {
		return err
	}
	if err := db.Exec(pgCreateTSVIndex).Error; err != nil {
		return err
	}
	return nil
}

func (s *PostgresBM25Store) createTrigger(ctx context.Context) error {
	db := s.db.WithContext(ctx)

	if err := db.Exec(pgCreateTriggerFunction).Error; err != nil {
		return err
	}
	if err := db.Exec(pgCreateTrigger).Error; err != nil {
		return err
	}
	return nil
}

func (s *PostgresBM25Store) existingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var existingIDs []string
	err := s.db.WithContext(ctx).Raw(pgCheckExistingIDsQuery, ids).Scan(&existingIDs).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		result[id] = struct{}{}
	}
	return result, nil
}

// Index adds documents to the BM25 index.
func (s *PostgresBM25Store) Index(ctx context.Context, request search.IndexRequest) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}

	documents := request.Documents()

	// Filter out invalid documents
	var valid []search.Document
	for _, doc := range documents {
		if doc.SnippetID() != "" && doc.Text() != "" {
			valid = append(valid, doc)
		}
	}

	if len(valid) == 0 {
		s.logger.Warn("corpus is empty, skipping bm25 index")
		return nil
	}

	// Filter out already indexed documents
	ids := make([]string, len(valid))
	for i, doc := range valid {
		ids[i] = doc.SnippetID()
	}

	existing, err := s.existingIDs(ctx, ids)
	if err != nil {
		return err
	}

	var toIndex []search.Document
	for _, doc := range valid {
		if _, exists := existing[doc.SnippetID()]; !exists {
			toIndex = append(toIndex, doc)
		}
	}

	if len(toIndex) == 0 {
		s.logger.Info("no new documents to index")
		return nil
	}

	// Execute inserts in a transaction
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, doc := range toIndex {
			if err := tx.Exec(pgInsertQuery, doc.SnippetID(), doc.Text()).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Find performs BM25-style keyword search using PostgreSQL ts_rank_cd.
func (s *PostgresBM25Store) Find(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	if err := s.initialize(ctx); err != nil {
		return nil, err
	}

	q := repository.Build(options...)
	query, ok := search.QueryFrom(q)
	if !ok || query == "" {
		return []search.Result{}, nil
	}

	limit := q.LimitValue()
	if limit <= 0 {
		limit = 10
	}

	sanitizedQuery := sanitizePostgresQuery(query)

	tx := s.db.WithContext(ctx).
		Table("postgres_bm25_documents").
		Select("snippet_id, ts_rank_cd(tsv, plainto_tsquery('english', ?)) as score", sanitizedQuery).
		Where("tsv @@ plainto_tsquery('english', ?)", sanitizedQuery)

	if snippetIDs := search.SnippetIDsFrom(q); len(snippetIDs) > 0 {
		tx = tx.Where("snippet_id IN ?", snippetIDs)
	}
	if filters, ok := search.FiltersFrom(q); ok {
		tx = applySearchFilters(tx, filters)
	}

	tx = tx.Order("score DESC").Limit(limit)

	var rows []struct {
		SnippetID string  `gorm:"column:snippet_id"`
		Score     float64 `gorm:"column:score"`
	}
	if err := tx.Scan(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]search.Result, len(rows))
	for i, row := range rows {
		results[i] = search.NewResult(row.SnippetID, row.Score)
	}

	return results, nil
}

// DeleteBy removes documents matching the given options.
func (s *PostgresBM25Store) DeleteBy(ctx context.Context, options ...repository.Option) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}

	q := repository.Build(options...)
	ids := search.SnippetIDsFrom(q)
	if len(ids) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Exec(pgDeleteQuery, ids).Error
}

// sanitizePostgresQuery removes characters that could cause issues with plainto_tsquery.
func sanitizePostgresQuery(query string) string {
	// Replace characters that might cause issues
	replacer := strings.NewReplacer(
		"'", " ",
		"\"", " ",
		"(", " ",
		")", " ",
		":", " ",
		"!", " ",
		"&", " ",
		"|", " ",
	)
	return replacer.Replace(query)
}
