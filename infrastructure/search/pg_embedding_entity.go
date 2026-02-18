package search

// PgEmbeddingEntity is a GORM model for PostgreSQL vector embedding tables.
// Both VectorChord and pgvector stores share this schema (id, snippet_id,
// embedding VECTOR(N)). Table routing is done via .Table(name) at the call
// site because GORM caches schemas by type and dynamic TableName() does not
// work across multiple table names for the same struct type.
type PgEmbeddingEntity struct {
	ID        int64    `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string   `gorm:"column:snippet_id;uniqueIndex"`
	Embedding PgVector `gorm:"column:embedding;type:vector"`
}

// newPgEmbeddingEntity creates a PgEmbeddingEntity ready for insertion.
func newPgEmbeddingEntity(snippetID string, embedding PgVector) PgEmbeddingEntity {
	return PgEmbeddingEntity{
		SnippetID: snippetID,
		Embedding: embedding,
	}
}
