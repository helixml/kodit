# Implementation Tasks

## Domain layer

- [ ] Add `VisionEmbedder` interface and `Image` type to `domain/search/embedder.go`
- [ ] Add `NamedEmbedder` struct to `domain/search/embedder.go` (pairs name + text/vision capability)
- [ ] Add `EmbedderName` field to `search.Embedding` and `search.Result`
- [ ] Add `WithEmbedderName(name string)` repository option in `domain/search/options.go`

## Provider layer

- [ ] Add `VisionEmbedder` interface and `VisionEmbeddingRequest` type to `infrastructure/provider/provider.go`
- [ ] Create `infrastructure/provider/openai_vision.go` — `OpenAIVisionEmbedder` using an OpenAI-compatible endpoint
- [ ] Add `VisionEmbedderAdapter` in `kodit.go` bridging `provider.VisionEmbedder` → `search.VisionEmbedder`

## Storage layer

- [ ] Add `embedder_name` column (default `"default"`) to the SQLite embedding model structs; let AutoMigrate apply
- [ ] Update SQLite store `Find`, `Search`, `Exists`, `DeleteBy` to filter by `WithEmbedderName` when set
- [ ] Update VectorChord store to derive table name from embedder name (e.g. `kodit_code_embeddings_<name>`) so different vector dimensions coexist

## Configuration & wiring

- [ ] Add `EmbedderConfig` struct and `EmbedderType` enum to `options.go`
- [ ] Add `WithEmbedders([]EmbedderConfig)` SDK option; register each as a named embedder
- [ ] Update `kodit.go` `New()` to iterate `EmbedderConfig` slice and create one `EmbeddingService` + `EmbeddingStore` per embedder
- [ ] Ensure backwards compatibility: `WithOpenAI`, `WithEmbeddingProvider` still register a `"default"` text embedder

## Indexing pipeline

- [ ] Update `application/handler/indexing/create_embeddings.go` to iterate all registered text embedders
- [ ] Create `application/handler/indexing/create_image_embeddings.go` — new handler iterating vision embedders over image blobs

## Search

- [ ] Propagate `EmbedderName` through `search.Result` to the API response
- [ ] Update `application/service/search.go` to support `WithEmbedderName` option — restricts vector search to one embedder's store

## Tests

- [ ] Unit test `NamedEmbedder` routing: text docs go to text embedders, images go to vision embedders
- [ ] Unit test SQLite store `WithEmbedderName` filtering
- [ ] Integration test: register two text embedders + one vision embedder; verify each stores and retrieves from its own namespace
- [ ] Verify backwards compatibility: single-embedder setup still works with no config changes
