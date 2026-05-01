package search

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// Store persists searchable documents and supports ranked retrieval.
// Both BM25 keyword stores and vector embedding stores share this contract.
//
// Index is bespoke per implementation (FTS5 INSERT, vchord_bm25 tokenize,
// or pgvector upsert). All other operations go through the embedded
// database.Repository so query/delete/exist semantics stay consistent.
//
// Find returns ranked results when WithQuery / WithEmbedding is supplied,
// or a plain lookup (Result.Score == 0) when neither is present.
type Store interface {
	Index(ctx context.Context, docs []Document) error
	Find(ctx context.Context, opts ...repository.Option) ([]Result, error)
	Count(ctx context.Context, opts ...repository.Option) (int64, error)
	Exists(ctx context.Context, opts ...repository.Option) (bool, error)
	DeleteBy(ctx context.Context, opts ...repository.Option) error
}
