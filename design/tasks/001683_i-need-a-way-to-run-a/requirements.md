# Requirements: Image Search

## Background

Kodit currently searches code and text. Users want to search repository images (diagrams, screenshots, UI mockups) by describing what they're looking for. This requires a new search type — image search — backed by a vision embedding model that runs alongside the existing text embedder.

## User Stories

**US-1: Search images by describing them**
As a developer, I want to type a description (e.g. "login form wireframe") and get back matching images from my repository, so that I can find visual assets without remembering filenames.

**US-2: Index images automatically**
As a developer, I want images committed to my repository to be indexed automatically when the repository is synced, so that I don't have to manage a separate image catalogue.

**US-3: Get image results alongside code results**
As a developer, I want image search results to link back to the file in the repository, so that I can navigate directly to the asset.

## Acceptance Criteria

- **AC-1** `client.Search.QueryImages(ctx, "description")` returns a ranked list of image results.
- **AC-2** Each image result includes the file path and repository reference.
- **AC-3** Images (JPEG, PNG, WebP) found during indexing are embedded using a configurable vision embedding model.
- **AC-4** `kodit.New()` accepts a `WithVisionProvider(...)` option to supply the vision embedding model; image indexing is skipped if none is configured.
- **AC-5** The vision embedder is a separate provider from the text embedder; the two run independently.
- **AC-6** Image embeddings are stored separately from code/text embeddings (no dimension conflicts).
- **AC-7** Existing text search behaviour is unchanged.
