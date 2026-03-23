# Requirements: Pipeline Preset Options for the Go API

## Background

Kodit is being used as a RAG (Retrieval-Augmented Generation) provider. In this use case, LLM-based enrichments (wiki generation, architecture discovery, cookbook, commit descriptions, database schema) are undesirable — they slow down indexing, require a text provider, and produce content that RAG consumers don't need. There is currently no way to suppress these operations via the public Go API.

## User Stories

### US-1: RAG-only indexing
As a developer integrating Kodit as a RAG backend, I want to skip all LLM-based enrichments during indexing so that indexing is fast and I don't need to configure a text provider.

**Acceptance Criteria:**
- `WithRAGPipeline()` option exists in the `kodit` package
- When set, the following operations are excluded from the pipeline: `CreateCommitDescription`, `CreateArchitectureEnrichment`, `CreateDatabaseSchema`, `CreateCookbook`, `GenerateWiki`
- `WithRAGPipeline()` works whether or not a text provider is configured
- A text provider can still be provided (e.g. for future use) without triggering enrichments
- No validation error is returned when `WithRAGPipeline()` is combined with a text provider

### US-2: Explicit full pipeline
As a developer who wants all enrichments, I want to explicitly declare `WithFullPipeline()` so that my intent is clear and the behaviour is stable regardless of future defaults.

**Acceptance Criteria:**
- `WithFullPipeline()` option exists in the `kodit` package
- When set (with a text provider configured), all LLM enrichments run — same as current default
- When set without a text provider, returns an error (same as current behavior)

### US-3: Default backward compatibility
As an existing Kodit user, I want the default behavior to be unchanged so that I don't need to update my code.

**Acceptance Criteria:**
- If neither `WithRAGPipeline()` nor `WithFullPipeline()` is passed, behavior is identical to today
- Enrichments run if a text provider is configured, are skipped otherwise

## Out of Scope

- Fine-grained per-operation toggles (e.g. skip only wiki)
- Custom pipeline operation ordering
- Pipeline presets beyond `Default`, `Full`, and `RAGOnly`
