package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

const (
	vchordBM25Table = "vectorchord_bm25_documents"

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

	bm25UpdateEmbeddingsQuery = `
UPDATE vectorchord_bm25_documents
SET embedding = tokenize(passage, 'bert')
WHERE embedding IS NULL`
)

const bm25BatchSize = 100

// ErrBM25InitializationFailed indicates VectorChord BM25 initialization failed.
var ErrBM25InitializationFailed = errors.New("failed to initialize VectorChord BM25 repository")

// VchordBM25Model maps the vectorchord_bm25_documents table.
// Score is populated by the bm25 distance operator during ranked queries.
type VchordBM25Model struct {
	ID        int64   `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string  `gorm:"column:snippet_id;uniqueIndex"`
	Passage   string  `gorm:"column:passage"`
	Score     float64 `gorm:"->;-:migration"`
}

// TableName returns the BM25 documents table name.
func (VchordBM25Model) TableName() string { return vchordBM25Table }

// vchordBM25Mapper maps VchordBM25Model to search.Result.
// vchord_bm25 returns negative distances (more negative = better match);
// we negate to keep Result.Score positive for cross-store consistency.
type vchordBM25Mapper struct{}

func (vchordBM25Mapper) ToDomain(e VchordBM25Model) search.Result {
	return search.NewResult(e.SnippetID, -e.Score)
}

func (vchordBM25Mapper) ToModel(r search.Result) VchordBM25Model {
	return VchordBM25Model{SnippetID: r.SnippetID()}
}

// VectorChordBM25Store implements search.Store using VectorChord PostgreSQL extension.
type VectorChordBM25Store struct {
	database.Repository[search.Result, VchordBM25Model]
	db     *gorm.DB
	logger zerolog.Logger
}

// NewVectorChordBM25Store creates a new VectorChordBM25Store, eagerly initializing
// extensions, tokenizer, and tables.
func NewVectorChordBM25Store(db database.Database, logger zerolog.Logger) (*VectorChordBM25Store, error) {
	s := &VectorChordBM25Store{
		Repository: database.NewRepository[search.Result, VchordBM25Model](db, vchordBM25Mapper{}, "bm25 document"),
		db:         db.GORM(),
		logger:     logger,
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

// Find performs BM25 keyword search when WithQuery is supplied; otherwise
// delegates to the embedded Repository for plain snippet_id lookups.
func (s *VectorChordBM25Store) Find(ctx context.Context, opts ...repository.Option) ([]search.Result, error) {
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
		repository.WithSelect("snippet_id, embedding <&> to_bm25query('vectorchord_bm25_documents_idx', tokenize(?, 'bert')) AS score", query),
		repository.WithRawOrder("score ASC"),
		repository.WithLimit(limit),
	}
	if filters, ok := search.FiltersFrom(q); ok {
		augmented = append(augmented, filterJoinOptions(filters, "bigint")...)
	}
	if snippetIDs := search.SnippetIDsFrom(q); len(snippetIDs) > 0 {
		augmented = append(augmented, search.WithSnippetIDs(snippetIDs))
	}

	return s.Repository.Find(ctx, augmented...)
}

// Index adds documents to the BM25 index, then tokenizes the new rows.
//
// Filters out invalid (empty id or text) and already-indexed documents
// before INSERTing the remainder; duplicates would unnecessarily re-tokenize.
func (s *VectorChordBM25Store) Index(ctx context.Context, docs []search.Document) error {
	toIndex, err := s.newDocuments(ctx, docs)
	if err != nil {
		return err
	}
	if len(toIndex) == 0 {
		s.logger.Info().Msg("no new documents to index")
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.batchInsert(tx, toIndex); err != nil {
			return err
		}
		return tx.Exec(bm25UpdateEmbeddingsQuery).Error
	})
}

func (s *VectorChordBM25Store) newDocuments(ctx context.Context, docs []search.Document) ([]search.Document, error) {
	valid := make([]search.Document, 0, len(docs))
	for _, doc := range docs {
		if doc.SnippetID() != "" && doc.Text() != "" {
			valid = append(valid, doc)
		}
	}
	if len(valid) == 0 {
		s.logger.Warn().Msg("corpus is empty, skipping bm25 index")
		return nil, nil
	}

	ids := make([]string, len(valid))
	for i, doc := range valid {
		ids[i] = doc.SnippetID()
	}

	existing, err := search.ExistingSnippetIDs(ctx, s, ids)
	if err != nil {
		return nil, err
	}

	out := make([]search.Document, 0, len(valid))
	for _, doc := range valid {
		if _, dup := existing[doc.SnippetID()]; !dup {
			out = append(out, doc)
		}
	}
	return out, nil
}

func (s *VectorChordBM25Store) batchInsert(tx *gorm.DB, documents []search.Document) error {
	for start := 0; start < len(documents); start += bm25BatchSize {
		end := min(start+bm25BatchSize, len(documents))
		batch := documents[start:end]

		var b strings.Builder
		b.WriteString("INSERT INTO vectorchord_bm25_documents (snippet_id, passage, embedding) VALUES ")
		args := make([]any, 0, len(batch)*2)
		for i, doc := range batch {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("(?, ?, NULL)")
			args = append(args, doc.SnippetID(), doc.Text())
		}
		b.WriteString(" ON CONFLICT (snippet_id) DO UPDATE SET passage = EXCLUDED.passage, embedding = NULL")

		if err := tx.Exec(b.String(), args...).Error; err != nil {
			return err
		}
	}
	return nil
}
