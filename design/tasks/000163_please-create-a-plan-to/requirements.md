# Requirements: Handler Test Coverage

## Context

The `application/handler/` directory contains task handlers that form the core processing pipeline. Each handler implements `Execute(ctx, payload)` and follows a consistent pattern: extract payload, check for existing data (skip/idempotent), do work, persist results.

Tests should verify handlers from a user perspective — "I triggered this task; did the right data end up in the database?" — using a real SQLite test DB. The only mock is the LLM enricher (`fakeEnricher`).

## User Stories

**Repository handlers**

- As a user, when I clone a repository, the working copy path is persisted and a sync task is enqueued.
- As a user, if the repository is already cloned, the clone handler skips without error.
- As a user, when I delete a repository, the repo record, its enrichments, and the working copy on disk are all removed.
- As a user, when I sync a repository, branches are scanned and commit scan tasks are enqueued for the tracked branch.

**Enrichment handlers**

- As a user, when commit description runs for a commit, exactly one `TypeHistory/SubtypeCommitDescription` enrichment is created and associated with the commit SHA.
- As a user, if that enrichment already exists, the handler skips (idempotent).
- As a user, ExampleSummary creates one `SubtypeExampleSummary` per `SubtypeExample` enrichment and links them.
- As a user, ArchitectureDiscovery, Cookbook, DatabaseSchema, and APIDocs each produce exactly one enrichment of their respective type/subtype for the commit.
- As a user, ExtractExamples reads files from a real repo path and saves `SubtypeExample` enrichments.

**Indexing handlers**

- As a user, CreateBM25Index indexes chunk or snippet enrichments into the FTS5 store; subsequent runs skip already-indexed documents.
- As a user, CreateCodeEmbeddings sends enrichments to the embedding service; already-embedded enrichments are skipped.
- As a user, CreateExampleCodeEmbeddings and CreateExampleSummaryEmbeddings behave the same way for their respective subtypes.

## Acceptance Criteria

- All tests use `testdb.New(t)` (real SQLite with migrations).
- LLM calls go through a `fakeEnricher` that returns deterministic canned responses.
- Embedding and BM25 stores use real SQLite implementations (`NewSQLiteEmbeddingStore`, `NewSQLiteBM25Store`).
- Each test verifies the post-Execute state of the database, not internal method calls.
- `make test PKG=./application/handler/...` passes green.
- No test touches the network or filesystem beyond `t.TempDir()`.
