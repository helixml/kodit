# Implementation Tasks

## Domain

- [ ] Add `domain/search/vision_embedder.go` — `VisionEmbedder` interface (`EmbedImages`, `EmbedQuery`) and `Image` type
- [ ] Add `domain/search/image_result.go` — `ImageResult` and `ImageResults` types

## Provider

- [ ] Add `infrastructure/provider/vision.go` — `VisionEmbedding` interface and `OpenAIVisionEmbedder` implementation

## Storage

- [ ] Add `infrastructure/persistence/image_embedding_store_sqlite.go` — GORM model + SQLite store for `kodit_image_embeddings`
- [ ] Add `infrastructure/persistence/image_embedding_store_vectorchord.go` — VectorChord store for image embeddings

## Indexing

- [ ] Add `application/handler/indexing/create_image_embeddings.go` — walk indexed files, filter by image MIME type, call vision embedder, save embeddings

## Search

- [ ] Add `QueryImages(ctx, description string, opts ...SearchOption) (*ImageResults, error)` to `application/service/search.go`

## Wiring

- [ ] Add `WithVisionProvider(provider.VisionEmbedding)` option to `options.go`
- [ ] Wire vision provider, image embedding store, and image indexing handler in `kodit.go`; skip image indexing if no vision provider is configured

## Tests

- [ ] Unit test `OpenAIVisionEmbedder` (mock HTTP)
- [ ] Unit test `QueryImages` with a fake vision embedder and fake store
- [ ] Integration test: index an image file, query by description, assert result is returned
