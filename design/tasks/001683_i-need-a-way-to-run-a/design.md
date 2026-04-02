# Design: Image Search

## What We're Building

A new `QueryImages` method on `service.Search` that embeds a text description using a vision model and returns matching images from the repository. To support this, we need:

1. A vision embedder interface and an initial implementation (OpenAI-compatible endpoint).
2. An image indexing step that runs during repository sync.
3. A separate image embedding store (different vector dimension from text embedders).
4. A `WithVisionProvider` SDK option.

The existing text embedding pipeline is untouched.

## Key Design Decisions

### Vision embedder interface

Add `search.VisionEmbedder` alongside the existing `search.Embedder`:

```go
// VisionEmbedder converts image bytes into embedding vectors.
type VisionEmbedder interface {
    EmbedImages(ctx context.Context, images []Image) ([][]float64, error)
}

type Image struct {
    Data     []byte
    MIMEType string // "image/jpeg", "image/png", "image/webp"
}
```

Also add the symmetric capability to embed a text query into the vision embedding space — required so `QueryImages` can embed the user's description and compare it against image vectors:

```go
// EmbedQuery embeds a text description in the vision embedding space.
EmbedQuery(ctx context.Context, text string) ([]float64, error)
```

Both methods live on the same interface since a vision model typically handles both directions (text → vector, image → vector) in a shared embedding space (e.g. CLIP).

### Provider implementation

Add `provider.VisionEmbedding` interface in `infrastructure/provider/` and an initial `OpenAIVisionEmbedder` that calls any OpenAI-compatible endpoint (covers hosted CLIP, LLaVA-style APIs, or a local server). Configured via `WithVisionProvider(provider.VisionEmbedding)` option.

### Storage

Add a dedicated `ImageEmbeddingStore` (SQLite and VectorChord variants). Image embeddings are stored in separate tables (`kodit_image_embeddings`) so their vector dimension never conflicts with text embeddings. The table holds: `file_id`, `repo_id`, `path`, `embedding`.

### Indexing pipeline

Add a new `CreateImageEmbeddings` handler that runs during repository indexing. It walks indexed files, filters by MIME type (JPEG/PNG/WebP), reads the raw bytes, and calls the vision embedder. Skipped entirely if no vision provider is configured.

### Image search

Add `QueryImages(ctx, description string, opts ...SearchOption) (*ImageResults, error)` to `service.Search`. Steps:

1. Embed the description text using `VisionEmbedder.EmbedQuery`.
2. Call `ImageEmbeddingStore.Search` with the resulting vector.
3. Return ranked `ImageResult` values (file path, repo, similarity score).

### SDK option

```go
kodit.New(
    kodit.WithSQLite(".kodit/data.db"),
    kodit.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
    kodit.WithVisionProvider(provider.NewOpenAIVisionEmbedder(
        os.Getenv("VISION_API_KEY"),
        provider.WithVisionModel("clip-vit-base-patch32"),
    )),
)
```

## File Map

| File | Change |
|---|---|
| `domain/search/vision_embedder.go` | new: `VisionEmbedder`, `Image` |
| `domain/search/image_result.go` | new: `ImageResult`, `ImageResults` |
| `infrastructure/provider/vision.go` | new: `VisionEmbedding` interface, `OpenAIVisionEmbedder` |
| `infrastructure/persistence/image_embedding_store_sqlite.go` | new: SQLite image embedding store |
| `infrastructure/persistence/image_embedding_store_vectorchord.go` | new: VectorChord image embedding store |
| `application/handler/indexing/create_image_embeddings.go` | new: image indexing handler |
| `application/service/search.go` | add `QueryImages` method |
| `options.go` | add `WithVisionProvider` |
| `kodit.go` | wire vision provider + image embedding store |

## Patterns Found in Codebase

- GORM AutoMigrate only — define a new GORM model struct for `kodit_image_embeddings`.
- New options follow the `repository.WithCondition` pattern in `domain/<domain>/options.go`.
- `provider.Embedder` bridges to `search.Embedder` via an adapter in `kodit.go` — do the same for vision.
- Indexing handlers are registered as pipeline steps — the new image handler follows the same registration pattern.
- CLAUDE.md: no SQL migration files, no fallbacks, fail on missing configuration.
