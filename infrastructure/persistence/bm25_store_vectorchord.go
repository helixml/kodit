package persistence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

// SQL statements for VectorChord BM25 operations.
const (
	createPGTokenizer = `CREATE EXTENSION IF NOT EXISTS pg_tokenizer CASCADE`
	createVChordBM25  = `CREATE EXTENSION IF NOT EXISTS vchord_bm25 CASCADE`

	createBM25Table = `
CREATE TABLE IF NOT EXISTS vectorchord_bm25_documents (
    id SERIAL PRIMARY KEY,
    snippet_id VARCHAR(255) NOT NULL,
    passage TEXT NOT NULL,
    embedding bm25vector,
    UNIQUE(snippet_id)
)`

	createBM25Index = `
CREATE INDEX IF NOT EXISTS vectorchord_bm25_documents_idx
ON vectorchord_bm25_documents
USING bm25 (embedding bm25_ops)`

	tokenizerExistsQuery = `SELECT 1 FROM tokenizer_catalog.tokenizer WHERE name = 'bert'`

	loadTokenizer = `
SELECT create_tokenizer('bert', $$
model = "llmlingua2"
pre_tokenizer = "unicode_segmentation"
[[character_filters]]
to_lowercase = {}
[[character_filters]]
unicode_normalization = "nfkd"
[[token_filters]]
skip_non_alphanumeric = {}
[[token_filters]]
stopwords = "nltk_english"
[[token_filters]]
stemmer = "english_porter2"
$$)`

	bm25InsertQuery = `
INSERT INTO vectorchord_bm25_documents (snippet_id, passage, embedding)
VALUES (?, ?, NULL)
ON CONFLICT (snippet_id) DO UPDATE
SET passage = EXCLUDED.passage, embedding = NULL`

	bm25UpdateEmbeddingsQuery = `
UPDATE vectorchord_bm25_documents
SET embedding = tokenize(passage, 'bert')
WHERE embedding IS NULL`

	bm25DeleteQuery = `DELETE FROM vectorchord_bm25_documents WHERE snippet_id IN ?`

	bm25CheckExistingIDsQuery = `SELECT snippet_id FROM vectorchord_bm25_documents WHERE snippet_id IN ?`
)

// ErrBM25InitializationFailed indicates VectorChord BM25 initialization failed.
var ErrBM25InitializationFailed = errors.New("failed to initialize VectorChord BM25 repository")

// VectorChordBM25Store implements search.BM25Store using VectorChord PostgreSQL extension.
type VectorChordBM25Store struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewVectorChordBM25Store creates a new VectorChordBM25Store, eagerly initializing
// extensions, tokenizer, and tables.
func NewVectorChordBM25Store(db database.Database, logger *slog.Logger) (*VectorChordBM25Store, error) {
	if logger == nil {
		logger = slog.Default()
	}
	s := &VectorChordBM25Store{
		db:     db.GORM(),
		logger: logger,
	}

	ctx := context.Background()

	if err := s.createExtensions(ctx); err != nil {
		return nil, errors.Join(ErrBM25InitializationFailed, err)
	}

	if err := s.createTokenizerIfNotExists(ctx); err != nil {
		return nil, errors.Join(ErrBM25InitializationFailed, err)
	}

	if err := s.createTables(ctx); err != nil {
		return nil, errors.Join(ErrBM25InitializationFailed, err)
	}

	return s, nil
}

func (s *VectorChordBM25Store) createExtensions(ctx context.Context) error {
	db := s.db.WithContext(ctx)

	if err := db.Exec(vcCreateVChordExtension).Error; err != nil {
		return fmt.Errorf("create vchord extension: %w", err)
	}
	if err := db.Exec(createPGTokenizer).Error; err != nil {
		return fmt.Errorf("create pg_tokenizer extension: %w", err)
	}
	if err := db.Exec(createVChordBM25).Error; err != nil {
		return fmt.Errorf("create vchord_bm25 extension: %w", err)
	}
	return nil
}

func (s *VectorChordBM25Store) createTokenizerIfNotExists(ctx context.Context) error {
	db := s.db.WithContext(ctx)

	var exists int
	result := db.Raw(tokenizerExistsQuery).Scan(&exists)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	// Tokenizer doesn't exist if RowsAffected is 0
	if result.RowsAffected == 0 {
		if err := db.Exec(loadTokenizer).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *VectorChordBM25Store) createTables(ctx context.Context) error {
	db := s.db.WithContext(ctx)

	if err := db.Exec(createBM25Table).Error; err != nil {
		return fmt.Errorf("create bm25 table: %w", err)
	}
	if err := db.Exec(createBM25Index).Error; err != nil {
		return fmt.Errorf("create bm25 index: %w", err)
	}
	return nil
}

func (s *VectorChordBM25Store) existingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var found []string
	err := s.db.WithContext(ctx).Raw(bm25CheckExistingIDsQuery, ids).Scan(&found).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(found))
	for _, id := range found {
		result[id] = struct{}{}
	}
	return result, nil
}

// Index adds documents to the BM25 index.
func (s *VectorChordBM25Store) Index(ctx context.Context, request search.IndexRequest) error {
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
			if err := tx.Exec(bm25InsertQuery, doc.SnippetID(), doc.Text()).Error; err != nil {
				return err
			}
		}

		// Tokenize the new documents
		if err := tx.Exec(bm25UpdateEmbeddingsQuery).Error; err != nil {
			return err
		}

		return nil
	})
}

// Find performs BM25 keyword search using options.
func (s *VectorChordBM25Store) Find(ctx context.Context, options ...repository.Option) ([]search.Result, error) {
	q := repository.Build(options...)
	query, ok := search.QueryFrom(q)
	if !ok || query == "" {
		return []search.Result{}, nil
	}

	limit := q.LimitValue()
	if limit <= 0 {
		limit = 10
	}

	tx := s.db.WithContext(ctx).
		Table("vectorchord_bm25_documents").
		Select("snippet_id, embedding <&> to_bm25query('vectorchord_bm25_documents_idx', tokenize(?, 'bert')) AS bm25_score", query)

	if snippetIDs := search.SnippetIDsFrom(q); len(snippetIDs) > 0 {
		tx = tx.Where("snippet_id IN ?", snippetIDs)
	}
	if filters, ok := search.FiltersFrom(q); ok {
		tx = database.ApplySearchFilters(tx, filters)
	}

	tx = tx.Order("bm25_score").Limit(limit)

	var rows []struct {
		SnippetID string  `gorm:"column:snippet_id"`
		BM25Score float64 `gorm:"column:bm25_score"`
	}
	if err := tx.Scan(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]search.Result, len(rows))
	for i, row := range rows {
		// VectorChord returns negative scores (higher is better when more negative)
		// Convert to positive scores for consistency
		results[i] = search.NewResult(row.SnippetID, -row.BM25Score)
	}

	return results, nil
}

// DeleteBy removes documents matching the given options.
func (s *VectorChordBM25Store) DeleteBy(ctx context.Context, options ...repository.Option) error {
	q := repository.Build(options...)
	ids := search.SnippetIDsFrom(q)
	if len(ids) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Exec(bm25DeleteQuery, ids).Error
}
