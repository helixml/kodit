# Requirements: Kodit-based RAG

## Context

Helix uses pluggable RAG backends (`api/pkg/rag/`) to index and search knowledge. Existing backends include typesense, llamaindex, haystack, and pgvector. Kodit (the code-intelligence library) can now index plain local directories via `file://` URIs (commit `417f16b`). This task adds a kodit-based RAG implementation to helix that leverages kodit's own chunking, embedding, and search pipeline.

## User Stories

**As a helix operator**, I want to configure `RAG_DEFAULT_PROVIDER=kodit` so that knowledge from filestore sources is indexed and searched using kodit's semantic search pipeline instead of a separate vector store.

**As a helix operator**, I want kodit to index files directly from the local filestore path so that files don't need to be extracted and re-chunked by helix before indexing.

**As a helix user**, I want knowledge searches to use kodit's semantic search so that code and technical documents are retrieved with high relevance.

## Acceptance Criteria

1. `RAG_DEFAULT_PROVIDER=kodit` is a valid option; helix starts without error when set.
2. When a filestore-backed knowledge source is indexed with the kodit provider, kodit receives the local filesystem path via a `file://` URI and indexes it autonomously.
3. The kodit repository ID is persisted as metadata alongside the filestore path so it can be retrieved on future search and delete operations.
4. `Index(ctx, chunks...)` is a no-op for the kodit implementation (kodit handles chunking internally).
5. `Query` uses kodit's semantic search filtered to the repository for the knowledge source and returns results in `[]*types.SessionRAGResult` format.
6. `Delete` removes the kodit repository associated with the knowledge source.
7. `go.mod` in helix references the kodit commit that includes the `file://` local directory support (`417f16b`).
8. If the knowledge source is not a filestore (e.g., web), the kodit RAG implementation returns a clear error.
