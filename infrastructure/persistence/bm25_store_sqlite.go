package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

const (
	sqliteBM25Table = "kodit_bm25_documents"

	sqliteCreateFTS5Table = `
CREATE VIRTUAL TABLE IF NOT EXISTS kodit_bm25_documents USING fts5(
    snippet_id UNINDEXED,
    passage,
    tokenize='porter ascii'
)`

	sqliteInsertQuery = `
INSERT INTO kodit_bm25_documents (rowid, snippet_id, passage)
VALUES (?, ?, ?)`

	sqliteMaxRowIDQuery = `SELECT COALESCE(MAX(rowid), 0) FROM kodit_bm25_documents`
)

// ErrSQLiteBM25InitializationFailed indicates SQLite FTS5 initialization failed.
var ErrSQLiteBM25InitializationFailed = errors.New("failed to initialize SQLite FTS5 BM25 store")

// SQLiteBM25Model maps the FTS5 virtual table. Score is populated by the
// bm25() function during ranked queries; it is not a stored column.
type SQLiteBM25Model struct {
	SnippetID string  `gorm:"column:snippet_id"`
	Passage   string  `gorm:"column:passage"`
	Score     float64 `gorm:"->;-:migration"`
}

// TableName returns the FTS5 virtual table name.
func (SQLiteBM25Model) TableName() string { return sqliteBM25Table }

// sqliteBM25Mapper maps SQLiteBM25Model to search.Result.
// SQLite's bm25() returns negative scores (lower = better); we negate
// to keep Result.Score positive for cross-store consistency.
type sqliteBM25Mapper struct{}

func (sqliteBM25Mapper) ToDomain(e SQLiteBM25Model) search.Result {
	return search.NewResult(e.SnippetID, -e.Score)
}

func (sqliteBM25Mapper) ToModel(r search.Result) SQLiteBM25Model {
	return SQLiteBM25Model{SnippetID: r.SnippetID()}
}

// SQLiteBM25Store implements search.Store using SQLite FTS5.
type SQLiteBM25Store struct {
	database.Repository[search.Result, SQLiteBM25Model]
	logger    zerolog.Logger
	nextRowID int64
}

// NewSQLiteBM25Store creates a new SQLiteBM25Store, eagerly initializing
// the FTS5 table and row ID counter.
func NewSQLiteBM25Store(db database.Database, logger zerolog.Logger) (*SQLiteBM25Store, error) {
	s := &SQLiteBM25Store{
		Repository: database.NewRepository[search.Result, SQLiteBM25Model](db, sqliteBM25Mapper{}, "bm25 document"),
		logger:     logger,
	}

	ctx := context.Background()

	if err := s.DB(ctx).Exec(sqliteCreateFTS5Table).Error; err != nil {
		return nil, errors.Join(ErrSQLiteBM25InitializationFailed, fmt.Errorf("create fts5 table: %w", err))
	}

	var maxRowID int64
	if err := s.DB(ctx).Raw(sqliteMaxRowIDQuery).Scan(&maxRowID).Error; err != nil {
		return nil, errors.Join(ErrSQLiteBM25InitializationFailed, fmt.Errorf("read max rowid: %w", err))
	}
	s.nextRowID = maxRowID + 1

	return s, nil
}

// Find performs BM25 keyword search when WithQuery is supplied; otherwise
// delegates to the embedded Repository for plain snippet_id lookups
// (used by ExistingSnippetIDs and similar).
func (s *SQLiteBM25Store) Find(ctx context.Context, opts ...repository.Option) ([]search.Result, error) {
	q := repository.Build(opts...)
	query, ok := search.QueryFrom(q)
	if !ok || query == "" {
		return s.Repository.Find(ctx, opts...)
	}

	limit := q.LimitValue()
	if limit <= 0 {
		limit = 10
	}

	augmented := []repository.Option{
		repository.WithSelect("snippet_id, bm25(kodit_bm25_documents) AS score"),
		repository.WithWhere("kodit_bm25_documents MATCH ?", escapeFTS5Query(query)),
		repository.WithRawOrder("score ASC"),
		repository.WithLimit(limit),
	}
	augmented = appendSearchFilters(augmented, q, "INTEGER")

	return s.Repository.Find(ctx, augmented...)
}

// Index adds documents to the BM25 index.
//
// Filters out invalid (empty id or text) and already-indexed documents
// before INSERTing the remainder in a single transaction.
func (s *SQLiteBM25Store) Index(ctx context.Context, docs []search.Document) error {
	toIndex, err := filterNewDocuments(ctx, s, docs, s.logger)
	if err != nil {
		return err
	}
	if len(toIndex) == 0 {
		s.logger.Info().Msg("no new documents to index")
		return nil
	}

	return s.DB(ctx).Transaction(func(tx *gorm.DB) error {
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

// escapeFTS5Query escapes special characters for FTS5 queries.
func escapeFTS5Query(query string) string {
	// For simple queries, wrap in double quotes to treat as a phrase
	// FTS5 special chars: AND OR NOT ( ) * ^
	return "\"" + query + "\""
}
