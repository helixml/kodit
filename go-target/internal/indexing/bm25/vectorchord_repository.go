// Package bm25 provides BM25 full-text search implementations.
package bm25

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/helixml/kodit/internal/domain"
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

	insertQuery = `
INSERT INTO vectorchord_bm25_documents (snippet_id, passage)
VALUES (?, ?)
ON CONFLICT (snippet_id) DO UPDATE
SET passage = EXCLUDED.passage`

	updateEmbeddingsQuery = `
UPDATE vectorchord_bm25_documents
SET embedding = tokenize(passage, 'bert')`

	searchQuery = `
SELECT
    snippet_id,
    embedding <&> to_bm25query('vectorchord_bm25_documents_idx', tokenize(?, 'bert')) AS bm25_score
FROM vectorchord_bm25_documents
ORDER BY bm25_score
LIMIT ?`

	searchQueryWithFilter = `
SELECT
    snippet_id,
    embedding <&> to_bm25query('vectorchord_bm25_documents_idx', tokenize(?, 'bert')) AS bm25_score
FROM vectorchord_bm25_documents
WHERE snippet_id IN ?
ORDER BY bm25_score
LIMIT ?`

	deleteQuery = `DELETE FROM vectorchord_bm25_documents WHERE snippet_id IN ?`

	checkExistingIDsQuery = `SELECT snippet_id FROM vectorchord_bm25_documents WHERE snippet_id IN ?`
)

// ErrInitializationFailed indicates VectorChord initialization failed.
var ErrInitializationFailed = errors.New("failed to initialize VectorChord repository")

// VectorChordRepository implements BM25Repository using VectorChord PostgreSQL extension.
type VectorChordRepository struct {
	db          *gorm.DB
	logger      *slog.Logger
	initialized bool
	mu          sync.Mutex
}

// NewVectorChordRepository creates a new VectorChordRepository.
func NewVectorChordRepository(db *gorm.DB, logger *slog.Logger) *VectorChordRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &VectorChordRepository{
		db:     db,
		logger: logger,
	}
}

// initialize sets up the VectorChord environment.
func (r *VectorChordRepository) initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return nil
	}

	if err := r.createExtensions(ctx); err != nil {
		return errors.Join(ErrInitializationFailed, err)
	}

	if err := r.createTokenizerIfNotExists(ctx); err != nil {
		return errors.Join(ErrInitializationFailed, err)
	}

	if err := r.createTables(ctx); err != nil {
		return errors.Join(ErrInitializationFailed, err)
	}

	r.initialized = true
	return nil
}

func (r *VectorChordRepository) createExtensions(ctx context.Context) error {
	db := r.db.WithContext(ctx)

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

func (r *VectorChordRepository) createTokenizerIfNotExists(ctx context.Context) error {
	db := r.db.WithContext(ctx)

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

func (r *VectorChordRepository) createTables(ctx context.Context) error {
	db := r.db.WithContext(ctx)

	if err := db.Exec(createBM25Table).Error; err != nil {
		return err
	}
	if err := db.Exec(createBM25Index).Error; err != nil {
		return err
	}
	return nil
}

func (r *VectorChordRepository) existingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var existingIDs []string
	err := r.db.WithContext(ctx).Raw(checkExistingIDsQuery, ids).Scan(&existingIDs).Error
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
func (r *VectorChordRepository) Index(ctx context.Context, request domain.IndexRequest) error {
	if err := r.initialize(ctx); err != nil {
		return err
	}

	documents := request.Documents()

	// Filter out invalid documents
	var valid []domain.Document
	for _, doc := range documents {
		if doc.SnippetID() != "" && doc.Text() != "" {
			valid = append(valid, doc)
		}
	}

	if len(valid) == 0 {
		r.logger.Warn("corpus is empty, skipping bm25 index")
		return nil
	}

	// Filter out already indexed documents
	ids := make([]string, len(valid))
	for i, doc := range valid {
		ids[i] = doc.SnippetID()
	}

	existing, err := r.existingIDs(ctx, ids)
	if err != nil {
		return err
	}

	var toIndex []domain.Document
	for _, doc := range valid {
		if _, exists := existing[doc.SnippetID()]; !exists {
			toIndex = append(toIndex, doc)
		}
	}

	if len(toIndex) == 0 {
		r.logger.Info("no new documents to index")
		return nil
	}

	// Execute inserts in a transaction
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, doc := range toIndex {
			if err := tx.Exec(insertQuery, doc.SnippetID(), doc.Text()).Error; err != nil {
				return err
			}
		}

		// Tokenize the new documents
		if err := tx.Exec(updateEmbeddingsQuery).Error; err != nil {
			return err
		}

		return nil
	})
}

// Search performs BM25 keyword search.
func (r *VectorChordRepository) Search(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error) {
	if err := r.initialize(ctx); err != nil {
		return nil, err
	}

	query := request.Query()
	if query == "" {
		return []domain.SearchResult{}, nil
	}

	topK := request.TopK()
	if topK <= 0 {
		topK = 10
	}

	var rows []struct {
		SnippetID  string  `gorm:"column:snippet_id"`
		BM25Score  float64 `gorm:"column:bm25_score"`
	}

	var err error
	snippetIDs := request.SnippetIDs()
	if len(snippetIDs) > 0 {
		err = r.db.WithContext(ctx).Raw(searchQueryWithFilter, query, snippetIDs, topK).Scan(&rows).Error
	} else {
		err = r.db.WithContext(ctx).Raw(searchQuery, query, topK).Scan(&rows).Error
	}

	if err != nil {
		return nil, err
	}

	results := make([]domain.SearchResult, len(rows))
	for i, row := range rows {
		// VectorChord returns negative scores (higher is better when more negative)
		// Convert to positive scores for consistency
		results[i] = domain.NewSearchResult(row.SnippetID, -row.BM25Score)
	}

	return results, nil
}

// Delete removes documents from the BM25 index.
func (r *VectorChordRepository) Delete(ctx context.Context, request domain.DeleteRequest) error {
	if err := r.initialize(ctx); err != nil {
		return err
	}

	ids := request.SnippetIDs()
	if len(ids) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Exec(deleteQuery, ids).Error
}
