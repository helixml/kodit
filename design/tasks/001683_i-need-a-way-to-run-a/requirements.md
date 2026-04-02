# Requirements: Multiple Specialised Embedding Models

## Background

Kodit currently supports a single embedding model at a time (text-only). Users need to run multiple specialised embedders side-by-side — including vision embedders that produce vectors from images — without a hard upper limit on the number of registered embedders.

## User Stories

**US-1: Search images by visual similarity**
As a developer, I want to search my repository's images (diagrams, screenshots, UI mockups) by describing what I'm looking for, so that I can find visually relevant assets without remembering exact filenames or alt text.

**US-2: Better search results per content type**
As a developer, I want code search and documentation search to each use a model trained for that content type, so that results are more relevant than using a single general-purpose model for everything.

**US-3: Add specialised embedders without rebuilding**
As a developer, I want to introduce a new specialised embedder (e.g. for audio transcripts or diagrams) without disrupting existing embedding pipelines, so that I can extend coverage incrementally.

**US-4: Know which embedder produced a result**
As a developer, I want search results to tell me which embedding model matched them, so that I can understand why a result was returned and tune my query accordingly.

**US-5: Target a query at a specific embedder**
As a developer, I want to direct a search query at one specific embedder (e.g. vision-only), so that I get back only the results that are semantically meaningful for that modality.

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
