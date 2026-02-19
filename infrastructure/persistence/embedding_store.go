package persistence

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

// TaskName represents the type of embeddings (code or text).
type TaskName string

// TaskName values.
var (
	TaskNameCode = TaskName("code")
	TaskNameText = TaskName("text")
)

// PgEmbeddingModel is a GORM model for PostgreSQL vector embedding tables.
type PgEmbeddingModel struct {
	ID        int64             `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string            `gorm:"column:snippet_id;uniqueIndex"`
	Embedding database.PgVector `gorm:"column:embedding;type:vector"`
}

// pgEmbeddingMapper maps between search.Embedding and PgEmbeddingModel.
type pgEmbeddingMapper struct{}

func (pgEmbeddingMapper) ToDomain(entity PgEmbeddingModel) search.Embedding {
	return search.NewEmbedding(entity.SnippetID, entity.Embedding.Floats())
}

func (pgEmbeddingMapper) ToModel(domain search.Embedding) PgEmbeddingModel {
	return PgEmbeddingModel{
		SnippetID: domain.SnippetID(),
		Embedding: database.NewPgVector(domain.Vector()),
	}
}

// Float64Slice is a custom type for JSON serialization of []float64 in SQLite.
type Float64Slice []float64

// Scan implements sql.Scanner for reading JSON from SQLite.
func (f *Float64Slice) Scan(value any) error {
	if value == nil {
		*f = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into Float64Slice", value)
	}

	return json.Unmarshal(data, f)
}

// Value implements driver.Valuer for writing JSON to SQLite.
func (f Float64Slice) Value() (driver.Value, error) {
	if f == nil {
		return nil, nil
	}
	return json.Marshal(f)
}

// SQLiteEmbeddingModel represents a vector embedding in SQLite.
type SQLiteEmbeddingModel struct {
	ID        int64        `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string       `gorm:"column:snippet_id;uniqueIndex"`
	Embedding Float64Slice `gorm:"column:embedding;type:json"`
}

// sqliteEmbeddingMapper maps between search.Embedding and SQLiteEmbeddingModel.
type sqliteEmbeddingMapper struct{}

func (sqliteEmbeddingMapper) ToDomain(entity SQLiteEmbeddingModel) search.Embedding {
	return search.NewEmbedding(entity.SnippetID, []float64(entity.Embedding))
}

func (sqliteEmbeddingMapper) ToModel(domain search.Embedding) SQLiteEmbeddingModel {
	vec := domain.Vector()
	cp := make(Float64Slice, len(vec))
	copy(cp, vec)
	return SQLiteEmbeddingModel{
		SnippetID: domain.SnippetID(),
		Embedding: cp,
	}
}

// cosineSearch performs a cosine-distance similarity search against a PG vector
// table and returns results sorted by similarity (highest first).
func cosineSearch(
	db *gorm.DB,
	tableName string,
	options ...repository.Option,
) ([]search.Result, error) {
	q := repository.Build(options...)
	embedding, ok := search.EmbeddingFrom(q)
	if !ok || len(embedding) == 0 {
		return []search.Result{}, nil
	}

	limit := q.LimitValue()
	if limit <= 0 {
		limit = 10
	}

	queryEmbedding := database.NewPgVector(embedding).String()

	tx := db.Table(tableName).
		Select("snippet_id, embedding <=> ? as score", queryEmbedding)
	tx = database.ApplyConditions(tx, options...)

	if filters, ok := search.FiltersFrom(q); ok {
		tx = database.ApplySearchFilters(tx, filters)
	}

	tx = tx.Order("score ASC").Limit(limit)

	var rows []struct {
		SnippetID string  `gorm:"column:snippet_id"`
		Score     float64 `gorm:"column:score"`
	}
	if err := tx.Scan(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]search.Result, len(rows))
	for i, row := range rows {
		// Cosine distance: 0 = identical, 2 = opposite.
		// Convert to similarity: 1 - distance/2 for 0-1 range.
		similarity := 1.0 - row.Score/2.0
		results[i] = search.NewResult(row.SnippetID, similarity)
	}

	return results, nil
}

// StoredVector holds an embedding vector with its snippet ID.
type StoredVector struct {
	snippetID string
	embedding []float64
}

// NewStoredVector creates a new StoredVector.
func NewStoredVector(snippetID string, embedding []float64) StoredVector {
	vec := make([]float64, len(embedding))
	copy(vec, embedding)
	return StoredVector{
		snippetID: snippetID,
		embedding: vec,
	}
}

// SnippetID returns the snippet identifier.
func (v StoredVector) SnippetID() string { return v.snippetID }

// Embedding returns the embedding vector (copy).
func (v StoredVector) Embedding() []float64 {
	result := make([]float64, len(v.embedding))
	copy(result, v.embedding)
	return result
}

// SimilarityMatch holds a snippet ID and its similarity score.
type SimilarityMatch struct {
	snippetID  string
	similarity float64
}

// NewSimilarityMatch creates a new SimilarityMatch.
func NewSimilarityMatch(snippetID string, similarity float64) SimilarityMatch {
	return SimilarityMatch{
		snippetID:  snippetID,
		similarity: similarity,
	}
}

// SnippetID returns the snippet identifier.
func (m SimilarityMatch) SnippetID() string { return m.snippetID }

// Similarity returns the similarity score.
func (m SimilarityMatch) Similarity() float64 { return m.similarity }

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 (opposite) and 1 (identical).
// Returns 0 if either vector has zero magnitude.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, magA, magB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}

	if magA == 0 || magB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(magA) * math.Sqrt(magB))
}

// TopKSimilar finds the top-k most similar vectors to the query.
// Returns results sorted by similarity in descending order (highest similarity first).
func TopKSimilar(query []float64, vectors []StoredVector, k int) []SimilarityMatch {
	if len(vectors) == 0 || k <= 0 {
		return []SimilarityMatch{}
	}

	matches := make([]SimilarityMatch, 0, len(vectors))
	for _, v := range vectors {
		similarity := CosineSimilarity(query, v.embedding)
		matches = append(matches, NewSimilarityMatch(v.snippetID, similarity))
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].similarity > matches[j].similarity
	})

	if k > len(matches) {
		k = len(matches)
	}
	return matches[:k]
}

// TopKSimilarFiltered finds the top-k most similar vectors, filtering by allowed snippet IDs.
func TopKSimilarFiltered(query []float64, vectors []StoredVector, k int, allowedIDs map[string]struct{}) []SimilarityMatch {
	if len(vectors) == 0 || k <= 0 {
		return []SimilarityMatch{}
	}

	if len(allowedIDs) == 0 {
		return TopKSimilar(query, vectors, k)
	}

	matches := make([]SimilarityMatch, 0, len(vectors))
	for _, v := range vectors {
		if _, ok := allowedIDs[v.snippetID]; !ok {
			continue
		}
		similarity := CosineSimilarity(query, v.embedding)
		matches = append(matches, NewSimilarityMatch(v.snippetID, similarity))
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].similarity > matches[j].similarity
	})

	if k > len(matches) {
		k = len(matches)
	}
	return matches[:k]
}
