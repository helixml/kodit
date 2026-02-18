package search

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Shared SQL templates for cosine-distance search. VectorChord and pgvector
// use the identical <=> operator so the queries are the same.
const (
	pgCosineSearchTemplate = `
SELECT snippet_id, embedding <=> ? as score
FROM %s
ORDER BY score ASC
LIMIT ?`

	pgCosineSearchWithFilterTemplate = `
SELECT snippet_id, embedding <=> ? as score
FROM %s
WHERE snippet_id IN ?
ORDER BY score ASC
LIMIT ?`
)

// identityMapper is an EntityMapper where D = E (entity IS the domain type).
// Embedding entities are purely infrastructure — no separate domain aggregate
// exists for them, so mapping is a no-op.
type identityMapper[E any] struct{}

func (identityMapper[E]) ToDomain(entity E) E { return entity }
func (identityMapper[E]) ToModel(domain E) E  { return domain }

// EntityFactory creates a GORM-insertable entity from a snippet ID and raw
// embedding vector. Each store provides its own factory because PG stores
// use PgVector while SQLite stores use Float64Slice.
type EntityFactory func(snippetID string, embedding []float64) any

// pgEntityFactory creates a PgEmbeddingEntity for PostgreSQL stores.
// Returns a pointer because GORM's Create requires a pointer to the entity.
func pgEntityFactory(snippetID string, embedding []float64) any {
	e := newPgEmbeddingEntity(snippetID, NewPgVector(embedding))
	return &e
}

// sqliteEntityFactory creates a SQLiteEmbeddingEntity for SQLite stores.
// Returns a pointer because GORM's Create requires a pointer to the entity.
func sqliteEntityFactory(snippetID string, embedding []float64) any {
	e := newSQLiteEmbeddingEntity(snippetID, embedding)
	return &e
}

// indexDocuments handles the full Index flow shared by all three vector stores:
// filter existing IDs → embed new documents → upsert in a transaction.
func indexDocuments(
	ctx context.Context,
	repo interface {
		DB(context.Context) *gorm.DB
		Table() string
	},
	embedder provider.Embedder,
	logger *slog.Logger,
	request search.IndexRequest,
	factory EntityFactory,
) error {
	documents := request.Documents()
	if len(documents) == 0 {
		return nil
	}

	db := repo.DB(ctx)
	tableName := repo.Table()

	// Collect snippet IDs to check for existing embeddings
	ids := make([]string, len(documents))
	for i, doc := range documents {
		ids[i] = doc.SnippetID()
	}

	existing, err := existingSnippetIDs(db, ids)
	if err != nil {
		return err
	}

	var toIndex []search.Document
	for _, doc := range documents {
		if _, ok := existing[doc.SnippetID()]; !ok {
			toIndex = append(toIndex, doc)
		}
	}

	if len(toIndex) == 0 {
		logger.Info("no new documents to index")
		return nil
	}

	// Get embeddings for new documents
	texts := make([]string, len(toIndex))
	for i, doc := range toIndex {
		texts[i] = doc.Text()
	}

	embResp, err := embedder.Embed(ctx, provider.NewEmbeddingRequest(texts))
	if err != nil {
		return fmt.Errorf("generate embeddings: %w", err)
	}

	embeddings := embResp.Embeddings()
	if len(embeddings) != len(toIndex) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(toIndex))
	}

	// Upsert in a transaction. Transactions do not inherit .Table() so we
	// must re-apply the table name inside the callback.
	return db.Transaction(func(tx *gorm.DB) error {
		for i, doc := range toIndex {
			entity := factory(doc.SnippetID(), embeddings[i])
			err := tx.Table(tableName).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "snippet_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"embedding"}),
			}).Create(entity).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// existingSnippetIDs returns a set of snippet IDs that already have rows in the
// table. The db parameter should already be table-scoped.
func existingSnippetIDs(db *gorm.DB, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}

	var found []string
	err := db.Where("snippet_id IN ?", ids).Pluck("snippet_id", &found).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(found))
	for _, id := range found {
		result[id] = struct{}{}
	}
	return result, nil
}

// cosineSearch performs a cosine-distance similarity search against a PG vector
// table and returns results sorted by similarity (highest first). Used by both
// VectorChord and pgvector stores.
func cosineSearch(
	ctx context.Context,
	db *gorm.DB,
	tableName string,
	embedder provider.Embedder,
	request search.Request,
) ([]search.Result, error) {
	query := request.Query()
	if query == "" {
		return []search.Result{}, nil
	}

	topK := request.TopK()
	if topK <= 0 {
		topK = 10
	}

	embResp, err := embedder.Embed(ctx, provider.NewEmbeddingRequest([]string{query}))
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	embeddings := embResp.Embeddings()
	if len(embeddings) == 0 {
		return []search.Result{}, nil
	}

	queryEmbedding := NewPgVector(embeddings[0]).String()

	var rows []struct {
		SnippetID string  `gorm:"column:snippet_id"`
		Score     float64 `gorm:"column:score"`
	}

	snippetIDs := request.SnippetIDs()
	if len(snippetIDs) > 0 {
		sql := fmt.Sprintf(pgCosineSearchWithFilterTemplate, tableName)
		err = db.Raw(sql, queryEmbedding, snippetIDs, topK).Scan(&rows).Error
	} else {
		sql := fmt.Sprintf(pgCosineSearchTemplate, tableName)
		err = db.Raw(sql, queryEmbedding, topK).Scan(&rows).Error
	}

	if err != nil {
		return nil, err
	}

	results := make([]search.Result, len(rows))
	for i, row := range rows {
		// Cosine distance: 0 = identical, 2 = opposite.
		// Convert to similarity: 1 - distance/2 for 0–1 range.
		similarity := 1.0 - row.Score/2.0
		results[i] = search.NewResult(row.SnippetID, similarity)
	}

	return results, nil
}

// newPgEmbeddingRepository creates a Repository for PgEmbeddingEntity with a
// dynamic table name. Used by both VectorChord and pgvector stores.
func newPgEmbeddingRepository(db database.Database, tableName string) database.Repository[PgEmbeddingEntity, PgEmbeddingEntity] {
	return database.NewRepositoryForTable[PgEmbeddingEntity, PgEmbeddingEntity](
		db,
		identityMapper[PgEmbeddingEntity]{},
		"embedding",
		tableName,
	)
}

// newSQLiteEmbeddingRepository creates a Repository for SQLiteEmbeddingEntity
// with a dynamic table name.
func newSQLiteEmbeddingRepository(db database.Database, tableName string) database.Repository[SQLiteEmbeddingEntity, SQLiteEmbeddingEntity] {
	return database.NewRepositoryForTable[SQLiteEmbeddingEntity, SQLiteEmbeddingEntity](
		db,
		identityMapper[SQLiteEmbeddingEntity]{},
		"embedding",
		tableName,
	)
}
