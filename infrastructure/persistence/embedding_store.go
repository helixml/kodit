package persistence

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/internal/database"
)

// saveAllBatchSize controls how many rows are inserted per multi-row INSERT
// for embedding stores.
const saveAllBatchSize = 100

// gitBatchSize controls how many git objects (commits, files, branches, tags)
// are inserted per batch. 1000 rows × 9 columns (worst case: TagModel) = 9 000
// bind parameters, well under PostgreSQL's 65 535-parameter limit.
const gitBatchSize = 1000

// TaskName represents the type of embeddings (code or text).
type TaskName string

// TaskName values.
var (
	TaskNameCode   = TaskName("code")
	TaskNameText   = TaskName("text")
	TaskNameVision = TaskName("vision")
)

// PgEmbeddingModel is a GORM model for PostgreSQL vector embedding tables.
// Score is populated transiently during ranked search (`embedding <=> ?`);
// it is never written.
type PgEmbeddingModel struct {
	ID        int64             `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string            `gorm:"column:snippet_id;uniqueIndex"`
	Embedding database.PgVector `gorm:"column:embedding;type:vector"`
	Score     float64           `gorm:"->;-:migration"`
}

// pgEmbeddingMapper maps PgEmbeddingModel to search.Result.
//
// pgvector's <=> operator returns cosine distance (0 = identical, 2 = opposite);
// we convert to a similarity in [0, 1] using `1 - distance/2`.
type pgEmbeddingMapper struct{}

func (pgEmbeddingMapper) ToDomain(e PgEmbeddingModel) search.Result {
	return search.NewResult(e.SnippetID, 1.0-e.Score/2.0)
}

func (pgEmbeddingMapper) ToModel(r search.Result) PgEmbeddingModel {
	return PgEmbeddingModel{SnippetID: r.SnippetID()}
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
// Score is unused for SQLite (similarity is computed in-memory in the
// store's Find override) but is included for symmetry with PgEmbeddingModel.
type SQLiteEmbeddingModel struct {
	ID        int64        `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string       `gorm:"column:snippet_id;uniqueIndex"`
	Embedding Float64Slice `gorm:"column:embedding;type:json"`
}

// sqliteEmbeddingMapper maps SQLiteEmbeddingModel to search.Result.
// SQLite's Find override populates Result.Score from in-memory similarity;
// the mapper is only invoked for plain (non-ranked) lookups, where score
// is not meaningful.
type sqliteEmbeddingMapper struct{}

func (sqliteEmbeddingMapper) ToDomain(e SQLiteEmbeddingModel) search.Result {
	return search.NewResult(e.SnippetID, 0)
}

func (sqliteEmbeddingMapper) ToModel(r search.Result) SQLiteEmbeddingModel {
	return SQLiteEmbeddingModel{SnippetID: r.SnippetID()}
}

// vectorRow is an embedding loaded from SQLite for in-memory similarity
// search — internal to SQLiteEmbeddingStore.
type vectorRow struct {
	snippetID string
	embedding []float64
}

// cosineSimilarity computes the cosine similarity of two vectors in [-1, 1].
// Returns 0 when lengths mismatch or either vector has zero magnitude.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

// topKSimilar returns the top-k rows by cosine similarity to query, sorted
// descending. When allowed is non-empty, rows whose snippetID is not in the
// set are skipped; an empty or nil set means no filter.
func topKSimilar(query []float64, rows []vectorRow, k int, allowed map[string]struct{}) []search.Result {
	if len(rows) == 0 || k <= 0 {
		return nil
	}
	matches := make([]search.Result, 0, len(rows))
	for _, r := range rows {
		if len(allowed) > 0 {
			if _, ok := allowed[r.snippetID]; !ok {
				continue
			}
		}
		matches = append(matches, search.NewResult(r.snippetID, cosineSimilarity(query, r.embedding)))
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Score() > matches[j].Score() })
	if k > len(matches) {
		k = len(matches)
	}
	return matches[:k]
}
