# Requirements: Vision Embedding Type

## Background

Kodit's embedding infrastructure is text-only. Before image search can be built, we need a vision embedding type — an interface and at least one implementation that can turn image bytes into vectors.

## User Story

**US-1: Configure a vision embedding model**
As a developer, I want to point kodit at a vision embedding model (via an OpenAI-compatible API) so that image bytes can be converted to embedding vectors, ready for a future image search feature.

## Acceptance Criteria

- **AC-1** A `VisionEmbedder` interface exists in the domain with a method that accepts image bytes and returns embedding vectors.
- **AC-2** An `OpenAIVisionEmbedder` provider implementation exists that calls an OpenAI-compatible endpoint.
- **AC-3** The vision embedder is configurable: base URL, model name, and API key.
- **AC-4** The existing text embedding interfaces and implementations are unchanged.
