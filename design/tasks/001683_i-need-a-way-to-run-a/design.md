# Design: Multiple Specialised Embedding Models

## Current Architecture

- `domain/search/embedder.go` — `Embedder` interface: `Embed(ctx, texts []string) ([][]float64, error)`
- `domain/service/embedding.go` — `EmbeddingService` holds one `search.Embedder`
- `kodit.go` — wires a single embedder (Hugot local model or OpenAI) into two `EmbeddingService` instances (code + enrichment text)
- `infrastructure/provider/provider.go` — `provider.Embedder` interface (text-only)
- Storage: SQLite and VectorChord stores hold one dimension per embedding type (code / text)

## Key Design Decisions

### 1. Add `VisionEmbedder` alongside `Embedder`

Rather than modifying the existing `Embedder` interface (which would break all existing implementations), add a separate interface in `domain/search/`:

```go
// VisionEmbedder converts image bytes into embedding vectors.
type VisionEmbedder interface {
    EmbedImages(ctx context.Context, images []Image) ([][]float64, error)
}

type Image struct {
    Data     []byte // raw bytes
    MIMEType string // "image/jpeg", "image/png", etc.
}
```

Future multimodal embedders can implement both `Embedder` and `VisionEmbedder`.

### 2. Named Embedder Registry

Introduce `search.NamedEmbedder` — a value type pairing a name with an embedder capability:

```go
type NamedEmbedder struct {
    Name    string       // unique, e.g. "code", "vision", "docs"
    Text    Embedder     // nil if vision-only
    Vision  VisionEmbedder // nil if text-only
}
```

The `Client` holds a slice of `NamedEmbedder` instead of a single embedder. The indexing pipeline iterates the slice and dispatches to appropriate embedders based on capability.

### 3. Storage Namespacing via Embedder Name

Add an `EmbedderName` field to `search.Embedding` and a corresponding `embedder_name` column in both storage backends.

- **SQLite:** Add `embedder_name TEXT NOT NULL DEFAULT 'default'` to both embedding tables. GORM AutoMigrate handles this.
- **VectorChord:** Create one table per embedder name (e.g. `kodit_code_embeddings_vision`) because different embedders may produce different vector dimensions. The store constructor receives the embedder name and derives the table name.

Add `repository.Option` `WithEmbedderName(name string)` for filtering/searching by embedder.

### 4. Per-Embedder `EmbeddingService`

Each `NamedEmbedder` gets its own `EmbeddingService` (domain service) and its own `EmbeddingStore` instance scoped to its name. The indexing handlers iterate over all registered text embedders; vision handlers iterate over vision embedders. This keeps each service simple with no internal routing logic.

### 5. Configuration: `WithEmbedders([]EmbedderConfig)`

Add a new SDK option that accepts a slice of embedder configs:

```go
type EmbedderConfig struct {
    Name        string
    Type        EmbedderType // EmbedderTypeText | EmbedderTypeVision
    Provider    provider.Embedder      // text
    VisionProv  provider.VisionEmbedder // vision
    Budget      search.TokenBudget
    Parallelism int
}
```

Backwards compatibility: `WithOpenAI`, `WithEmbeddingProvider` continue to register a single embedder under the name `"default"`.

### 6. Vision Provider

Add `provider.VisionEmbedder` interface in `infrastructure/provider/`:

```go
type VisionEmbedder interface {
    EmbedImages(ctx context.Context, req VisionEmbeddingRequest) (EmbeddingResponse, error)
}
```

Initial implementation: `OpenAIVisionEmbedder` using an OpenAI-compatible endpoint (e.g. CLIP via a local server or a hosted API). Accepts `base_url`, `model`, `api_key` via env or `EmbedderConfig`.

### 7. Search Results

`search.Result` already has a `SnippetID` and score. Extend it with `EmbedderName string` so callers know which embedder produced each hit. `Search.Query()` accepts an optional `WithEmbedderName(name)` option to restrict the search to one embedder's vectors.

## File Map

| File | Change |
|---|---|
| `domain/search/embedder.go` | add `VisionEmbedder`, `Image`, `NamedEmbedder` |
| `domain/search/store.go` | add `WithEmbedderName` option |
| `domain/search/result.go` | add `EmbedderName` to `Result` |
| `domain/service/embedding.go` | no change (single-embedder service stays) |
| `infrastructure/provider/provider.go` | add `VisionEmbedder`, `VisionEmbeddingRequest` |
| `infrastructure/provider/openai_vision.go` | new: `OpenAIVisionEmbedder` |
| `infrastructure/persistence/embedding_store_sqlite.go` | add `embedder_name` column |
| `infrastructure/persistence/embedding_store_vectorchord.go` | table name derived from embedder name |
| `options.go` | add `WithEmbedders([]EmbedderConfig)` |
| `kodit.go` | wire multiple `EmbeddingService` instances from registry |
| `application/handler/indexing/create_embeddings.go` | iterate text embedders |
| `application/handler/indexing/create_image_embeddings.go` | new: vision indexing handler |
| `application/service/search.go` | pass `EmbedderName` through; support `WithEmbedderName` filter |

## Patterns Found in Codebase

- GORM AutoMigrate only — no SQL migration files. Add columns by updating the GORM model struct.
- New options use `repository.WithCondition` pattern (see `domain/<domain>/options.go`).
- Provider adapters bridge `provider.X` → `search.X` (see `embeddingAdapter` in `kodit.go`).
- The `EmbeddingService` already supports progress callbacks and failure-rate thresholds; vision handler should reuse this.
- ONNX/Hugot uses a process-wide mutex — vision providers will not share this constraint.
- VectorChord auto-detects dimension at startup and rebuilds table if dimension changes; per-embedder tables make this safe.

## Constraints

- Vision embedders return different vector dimensions than text embedders; storage must not mix them.
- The existing `embed_model` build tag for Hugot must continue to work with no changes.
- All new provider types must implement `Close() error` (from `provider.Provider`).
