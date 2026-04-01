# Requirements: Multiple Specialised Embedding Models

## Background

Kodit currently supports a single embedding model at a time (text-only). Users need to run multiple specialised embedders side-by-side — including vision embedders that produce vectors from images — without a hard upper limit on the number of registered embedders.

## User Stories

**US-1: Multiple text embedders**
As a developer, I want to register more than one text embedding model so that I can use different models optimised for different content types (e.g. one for code, one for documentation prose).

**US-2: Vision embedding model**
As a developer, I want to register a vision embedding model so that image content (diagrams, screenshots) in my repositories can be indexed and searched by visual similarity.

**US-3: N embedders without limit**
As a developer, I want to register an arbitrary number of embedders (text, vision, or multimodal) so I can compose specialised pipelines without artificial constraints.

**US-4: Named embedder results in search**
As a developer, I want search results to indicate which embedder produced them so I can distinguish code-vector hits from vision-vector hits.

**US-5: Embedder-scoped search**
As a developer, I want to restrict a search query to a specific named embedder so that vision queries only hit vision embeddings and code queries only hit code embeddings.

## Acceptance Criteria

- **AC-1** `kodit.New()` accepts a list of named `EmbedderConfig` entries; zero or more may be vision type.
- **AC-2** Each embedder has a unique name used to namespace its embeddings in storage.
- **AC-3** Vision embedder accepts raw image bytes (JPEG/PNG/WebP) and returns `[][]float64` vectors.
- **AC-4** Embeddings in storage carry the embedder name; queries can filter by it.
- **AC-5** VectorChord store creates a separate table (or uses a discriminator column) per embedder so different vector dimensions coexist without conflicts.
- **AC-6** SQLite store stores the embedder name in the existing embedding tables.
- **AC-7** The indexing pipeline sends image blobs to registered vision embedders and text snippets to registered text embedders.
- **AC-8** Adding a new embedder type requires only: a new interface implementation + registration — no changes to existing embedders or the indexing handlers.
- **AC-9** Search results include the name of the embedder that produced each hit.
- **AC-10** Existing single-embedder configuration (env vars, `WithOpenAI`, `WithEmbeddingProvider`) continues to work unchanged (backwards compatibility).
