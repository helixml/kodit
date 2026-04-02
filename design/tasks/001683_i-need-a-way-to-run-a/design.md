# Design: Vision Embedding Type

## Scope

Add a vision embedding interface to the domain and one provider implementation. No pipeline, search, storage, or wiring changes — those come in follow-up tasks.

## Domain interface

Add `domain/search/vision_embedder.go`:

```go
// VisionEmbedder converts image bytes into embedding vectors.
type VisionEmbedder interface {
    // EmbedImages returns one vector per image.
    EmbedImages(ctx context.Context, images []Image) ([][]float64, error)
    // EmbedQuery embeds a text description in the vision embedding space,
    // so query vectors can be compared against image vectors (e.g. CLIP).
    EmbedQuery(ctx context.Context, text string) ([]float64, error)
}

type Image struct {
    Data     []byte
    MIMEType string // "image/jpeg", "image/png", "image/webp"
}
```

Both methods live on one interface because CLIP-style models share a single embedding space for images and text — separating them would make it impossible to compare a text query against image vectors.

## Provider interface and implementation

Add `infrastructure/provider/vision.go`:

- `VisionEmbedding` interface (mirrors the domain interface at the provider layer, consistent with how `provider.Embedder` mirrors `search.Embedder`)
- `OpenAIVisionEmbedder` struct implementing it, calling an OpenAI-compatible `/v1/embeddings` endpoint with base64-encoded image inputs
- Constructor: `NewOpenAIVisionEmbedder(baseURL, model, apiKey string) *OpenAIVisionEmbedder`

## Files

| File | Change |
|---|---|
| `domain/search/vision_embedder.go` | new: `VisionEmbedder` interface, `Image` type |
| `infrastructure/provider/vision.go` | new: `VisionEmbedding` interface, `OpenAIVisionEmbedder` |

## Patterns

- Provider interface at `infrastructure/provider/` mirrors domain interface at `domain/search/` — same pattern as `provider.Embedder` / `search.Embedder`.
- CLAUDE.md: no fallbacks, return errors, no panics.
