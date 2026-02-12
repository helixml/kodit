// Package search provides search infrastructure implementations for BM25 and vector search.
package search

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/helixml/kodit/domain/search"
	"gorm.io/gorm"
)

// SQL statements for VectorChord BM25 operations.
const (
	createVChordExtension = `CREATE EXTENSION IF NOT EXISTS vchord CASCADE`
	createPGTokenizer     = `CREATE EXTENSION IF NOT EXISTS pg_tokenizer CASCADE`
	createVChordBM25      = `CREATE EXTENSION IF NOT EXISTS vchord_bm25 CASCADE`
	setSearchPath         = `SET search_path TO "$user", public, bm25_catalog, pg_catalog, information_schema, tokenizer_catalog`

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
INSERT INTO vectorchord_bm25_documents (snippet_id, passage)
VALUES (?, ?)
ON CONFLICT (snippet_id) DO UPDATE
SET passage = EXCLUDED.passage`

	bm25UpdateEmbeddingsQuery = `
UPDATE vectorchord_bm25_documents
SET embedding = tokenize(passage, 'bert')`

	bm25SearchQuery = `
SELECT
    snippet_id,
    embedding <&> to_bm25query('vectorchord_bm25_documents_idx', tokenize(?, 'bert')) AS bm25_score
FROM vectorchord_bm25_documents
ORDER BY bm25_score
LIMIT ?`

	bm25SearchQueryWithFilter = `
SELECT
    snippet_id,
    embedding <&> to_bm25query('vectorchord_bm25_documents_idx', tokenize(?, 'bert')) AS bm25_score
FROM vectorchord_bm25_documents
WHERE snippet_id IN ?
ORDER BY bm25_score
LIMIT ?`

	bm25DeleteQuery = `DELETE FROM vectorchord_bm25_documents WHERE snippet_id IN ?`

	bm25CheckExistingIDsQuery = `SELECT snippet_id FROM vectorchord_bm25_documents WHERE snippet_id IN ?`
)

// ErrBM25InitializationFailed indicates VectorChord BM25 initialization failed.
var ErrBM25InitializationFailed = errors.New("failed to initialize VectorChord BM25 repository")

// VectorChordBM25Store implements search.BM25Store using VectorChord PostgreSQL extension.
type VectorChordBM25Store struct {
	db          *gorm.DB
	logger      *slog.Logger
	initialized bool
	mu          sync.Mutex
}

// NewVectorChordBM25Store creates a new VectorChordBM25Store.
func NewVectorChordBM25Store(db *gorm.DB, logger *slog.Logger) *VectorChordBM25Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &VectorChordBM25Store{
		db:     db,
		logger: logger,
	}
}

func (s *VectorChordBM25Store) initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	if err := s.createExtensions(ctx); err != nil {
		return errors.Join(ErrBM25InitializationFailed, err)
	}

	if err := s.createTokenizerIfNotExists(ctx); err != nil {
		return errors.Join(ErrBM25InitializationFailed, err)
	}

	if err := s.createTables(ctx); err != nil {
		return errors.Join(ErrBM25InitializationFailed, err)
	}

	s.initialized = true
	return nil
}

func (s *VectorChordBM25Store) createExtensions(ctx context.Context) error {
	db := s.db.WithContext(ctx)

	if err := db.Exec(createVChordExtension).Error; err != nil {
		return err
	}
	if err := db.Exec(createPGTokenizer).Error; err != nil {
		return err
	}
	if err := db.Exec(createVChordBM25).Error; err != nil {
		return err
	}
	if err := db.Exec(setSearchPath).Error; err != nil {
		return err
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
		return err
	}
	if err := db.Exec(createBM25Index).Error; err != nil {
		return err
	}
	return nil
}

func (s *VectorChordBM25Store) existingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var existingIDs []string
	err := s.db.WithContext(ctx).Raw(bm25CheckExistingIDsQuery, ids).Scan(&existingIDs).Error
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
func (s *VectorChordBM25Store) Index(ctx context.Context, request search.IndexRequest) error {
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

// Search performs BM25 keyword search.
func (s *VectorChordBM25Store) Search(ctx context.Context, request search.Request) ([]search.Result, error) {
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

	var rows []struct {
		SnippetID string  `gorm:"column:snippet_id"`
		BM25Score float64 `gorm:"column:bm25_score"`
	}

	var err error
	snippetIDs := request.SnippetIDs()
	if len(snippetIDs) > 0 {
		err = s.db.WithContext(ctx).Raw(bm25SearchQueryWithFilter, query, snippetIDs, topK).Scan(&rows).Error
	} else {
		err = s.db.WithContext(ctx).Raw(bm25SearchQuery, query, topK).Scan(&rows).Error
	}

	if err != nil {
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

// Delete removes documents from the BM25 index.
func (s *VectorChordBM25Store) Delete(ctx context.Context, request search.DeleteRequest) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}

	ids := request.SnippetIDs()
	if len(ids) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Exec(bm25DeleteQuery, ids).Error
}
