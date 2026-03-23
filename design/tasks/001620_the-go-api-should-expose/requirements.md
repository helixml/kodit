# Requirements: Pipeline Customisation via Go API

## Context

Kodit is being used as a RAG (Retrieval-Augmented Generation) provider. In this mode, the caller wants fast, low-cost indexing: chunks, BM25, and vector embeddings. The LLM-based enrichments (wiki generation, commit descriptions, architecture discovery, database schema, cookbook) add significant cost and latency and produce output that is irrelevant to pure RAG use cases.

## User Stories

**US1 — Disable all LLM enrichments**
> As a Go API user using Kodit as a RAG provider, I want to disable all LLM-based enrichments at client construction time, so that indexing only produces code chunks, BM25 indices, and vector embeddings without incurring LLM costs.

Acceptance criteria:
- `kodit.New(kodit.WithoutEnrichments(), ...)` creates a client that never enqueues enrichment operations (wiki, commit description, architecture, database schema, cookbook, summary embeddings)
- A text provider can still be configured alongside `WithoutEnrichments()` (e.g. for embedding generation via OpenAI) without triggering enrichments
- The existing default behaviour (enrichments enabled when a text provider is present) is unchanged for callers who don't use the new option

**US2 — No silent surprises**
> As a Go API user, I want the pipeline configuration to be explicit and readable at the call site, so I can understand what my client will and won't do without reading internal code.

Acceptance criteria:
- The option is named and documented clearly in `options.go`
- The log warning about missing enrichment endpoint is suppressed when the caller has explicitly opted out

## Out of Scope

- Per-enrichment-type toggles (e.g. disable only wiki but keep commit descriptions) — one flag covering all LLM enrichments is sufficient for the stated use case; finer control can be added later if needed
- HTTP API / environment variable configuration — this task is Go API only
