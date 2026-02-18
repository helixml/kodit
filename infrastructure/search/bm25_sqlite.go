package search

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"gorm.io/gorm"
)

// SQL statements for SQLite FTS5 BM25 operations.
const (
	sqliteCreateFTS5Table = `
CREATE VIRTUAL TABLE IF NOT EXISTS kodit_bm25_documents USING fts5(
    snippet_id UNINDEXED,
    passage,
    tokenize='porter ascii'
)`

	sqliteInsertQuery = `
INSERT INTO kodit_bm25_documents (rowid, snippet_id, passage)
VALUES (?, ?, ?)`

	sqliteDeleteQuery = `DELETE FROM kodit_bm25_documents WHERE snippet_id IN ?`

	sqliteCheckExistingQuery = `SELECT snippet_id FROM kodit_bm25_documents WHERE snippet_id IN ?`

	sqliteMaxRowIDQuery = `SELECT COALESCE(MAX(rowid), 0) FROM kodit_bm25_documents`
)

// ErrSQLiteBM25InitializationFailed indicates SQLite FTS5 initialization failed.
var ErrSQLiteBM25InitializationFailed = errors.New("failed to initialize SQLite FTS5 BM25 store")

// SQLiteBM25Store implements search.BM25Store using SQLite FTS5.
type SQLiteBM25Store struct {
	db          *gorm.DB
	logger      *slog.Logger
	initialized bool
	nextRowID   int64
	mu          sync.Mutex
}

// NewSQLiteBM25Store creates a new SQLiteBM25Store.
func NewSQLiteBM25Store(db *gorm.DB, logger *slog.Logger) *SQLiteBM25Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &SQLiteBM25Store{
		db:     db,
		logger: logger,
	}
}

func (s *SQLiteBM25Store) initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	if err := s.createTable(ctx); err != nil {
		return errors.Join(ErrSQLiteBM25InitializationFailed, err)
	}

	// Get the max rowid to continue from
	var maxRowID int64
	if err := s.db.WithContext(ctx).Raw(sqliteMaxRowIDQuery).Scan(&maxRowID).Error; err != nil {
		return errors.Join(ErrSQLiteBM25InitializationFailed, err)
	}
	s.nextRowID = maxRowID + 1

	s.initialized = true
	return nil
}

func (s *SQLiteBM25Store) createTable(ctx context.Context) error {
	return s.db.WithContext(ctx).Exec(sqliteCreateFTS5Table).Error
}

func (s *SQLiteBM25Store) existingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var existingIDs []string
	err := s.db.WithContext(ctx).Raw(sqliteCheckExistingQuery, ids).Scan(&existingIDs).Error
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
func (s *SQLiteBM25Store) Index(ctx context.Context, request search.IndexRequest) error {
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
			rowID := s.nextRowID
			s.nextRowID++
			if err := tx.Exec(sqliteInsertQuery, rowID, doc.SnippetID(), doc.Text()).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Find performs BM25 keyword search using options.
func (s *SQLiteBM25Store) Find(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
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

	ftsQuery := escapeF5Query(query)

	tx := s.db.WithContext(ctx).
		Table("kodit_bm25_documents").
		Select("snippet_id, bm25(kodit_bm25_documents) as score").
		Where("kodit_bm25_documents MATCH ?", ftsQuery)

	if snippetIDs := search.SnippetIDsFrom(q); len(snippetIDs) > 0 {
		tx = tx.Where("snippet_id IN ?", snippetIDs)
	}
	if filters, ok := search.FiltersFrom(q); ok {
		tx = applySearchFilters(tx, filters)
	}

	tx = tx.Order("score").Limit(limit)

	// Use manual row scanning to ensure FTS5 UNINDEXED columns are read correctly
	sqlRows, err := tx.Rows()
	if err != nil {
		return nil, err
	}
	defer func() { _ = sqlRows.Close() }()

	var results []search.Result
	for sqlRows.Next() {
		var snippetID string
		var score float64
		if err := sqlRows.Scan(&snippetID, &score); err != nil {
			return nil, err
		}
		// SQLite bm25() returns negative scores (lower/more negative is better)
		// Convert to positive scores for consistency (negate)
		results = append(results, search.NewResult(snippetID, -score))
	}

	if err := sqlRows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// DeleteBy removes documents matching the given options.
func (s *SQLiteBM25Store) DeleteBy(ctx context.Context, options ...repository.Option) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}

	q := repository.Build(options...)
	ids := search.SnippetIDsFrom(q)
	if len(ids) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Exec(sqliteDeleteQuery, ids).Error
}

// escapeF5Query escapes special characters for FTS5 queries.
func escapeF5Query(query string) string {
	// For simple queries, wrap in double quotes to treat as a phrase
	// FTS5 special chars: AND OR NOT ( ) * ^
	// Escape by wrapping in quotes
	return "\"" + query + "\""
}
