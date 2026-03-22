# Implementation Tasks

## Domain & Persistence

- [~] Add `FindAll(ctx context.Context, filters search.Filters) ([]search.Embedding, error)` to `search.EmbeddingStore` interface in `domain/search/store.go`
- [~] Implement `FindAll()` in `SQLiteEmbeddingStore` (`infrastructure/persistence/embedding_store_sqlite.go`) — reuse `loadVectors()`, convert to `[]search.Embedding`
- [~] Implement `FindAll()` in `VectorChordEmbeddingStore` (`infrastructure/persistence/embedding_store_vectorchord.go`) — raw GORM query with `database.ApplySearchFilters()`

## Application Service (TDD)

- [ ] Write failing tests in `application/service/duplicates_test.go`: empty input, identical vectors → sim=1.0, dissimilar vectors below threshold, threshold boundary (inclusive), limit cap, cross-repo pairs
- [ ] Create `application/service/duplicates.go` with `DuplicateSearch` struct and `FindDuplicates(ctx, repoIDs, threshold, limit) ([]DuplicatePair, bool, error)` — normalize vectors, triangle scan, sort, cap; return `truncated=true` if N > 5000
- [ ] Run tests until all pass (`make test PKG=./application/service/...`)

## API Layer

- [ ] Add DTOs in `infrastructure/api/v1/dto/duplicates.go`: `DuplicateSearchRequest`, `DuplicatesResponse`, `DuplicatePairData`, `DuplicatesMeta`
- [ ] Add `Duplicates(w, req)` handler to `SearchRouter` in `infrastructure/api/v1/search.go`
- [ ] Register route `router.Post("/duplicates", r.Duplicates)` in `SearchRouter.Routes()`
- [ ] Add Swagger annotations (`@Summary`, `@Description`, `@Accept`, `@Produce`, `@Param`, `@Success`, `@Failure`, `@Router`)

## E2E Tests

- [ ] Write `test/e2e/duplicates_test.go`:
  - `TestDuplicates_NoEmbeddings_ReturnsEmpty` — POST with valid repo ID, expect 200 + empty data
  - `TestDuplicates_MissingRepositoryIDs_Returns400`
  - `TestDuplicates_InvalidThreshold_Returns400` (threshold = 0, threshold = 1.5)
  - `TestDuplicates_InvalidLimit_Returns400` (limit = 0)
  - `TestDuplicates_WithSeededEmbeddings_ReturnsPairs` — seed two near-identical embeddings into SQLite code embedding table, assert pair is returned with similarity ≥ threshold
  - `TestDuplicates_BelowThreshold_ReturnsEmpty` — seed two dissimilar embeddings, set high threshold, expect empty
- [ ] Add `SeedCodeEmbedding(snippetID string, vec []float64)` helper to `TestServer` in `test/e2e/helpers_test.go`

## MCP Tool

- [ ] Add `kodit_find_duplicates` tool definition to `internal/mcp/catalog.go` with params: `repo_url` (required string), `threshold` (optional number), `limit` (optional number)
- [ ] Add `DuplicateFinder` interface to `internal/mcp/server.go`
- [ ] Add `duplicateFinder DuplicateFinder` field to `Server` struct; update `NewServer()` signature
- [ ] Implement `handleFindDuplicates()` handler — resolve repo URL to ID, call service, marshal pairs to JSON
- [ ] Register handler in `registerTools()`

## Wiring

- [ ] Add `Duplicates *service.DuplicateSearch` to `kodit.Client` struct in `kodit.go`
- [ ] Instantiate and wire `service.NewDuplicateSearch(codeEmbeddingStore, enrichmentStore, logger)` in `kodit.go`
- [ ] Pass `client.Duplicates` to `NewSearchRouter()` and `mcp.NewServer()`

## Validation & Final Checks

- [ ] Run full test suite: `make check`
- [ ] Manually test API with `curl` against `localhost:8080` (see design.md manual test plan)
- [ ] Verify MCP tool is listed in `client.MCPServer.Tools()`
