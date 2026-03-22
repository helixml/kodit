# Design: Duplicate Chunk Detection

## Architecture Overview

The feature follows the existing layered pattern used by search:

```
MCP Tool (internal/mcp/)
  └── API Handler (infrastructure/api/v1/search.go)
        └── Application Service (application/service/duplicates.go)
              └── EmbeddingStore.FindAll() (infrastructure/persistence/)
```

## Algorithm: Efficient Pairwise Similarity

### The Problem
Comparing all N embeddings against all others naively is O(N²·D) where D is the embedding dimension. For N=5000, D=768, this is ~19 billion multiply-adds — too slow without optimization.

### The Shortcut: Normalize Once, Dot-Product Only

Cosine similarity = (a · b) / (|a| |b|). If vectors are pre-normalized to unit length, cosine similarity reduces to a plain dot product. Normalize all N vectors once (O(N·D)), then all N*(N-1)/2 pairwise comparisons are dot products (no sqrt per pair).

**Go implementation:**
```
1. Load all embeddings for the repos → []StoredVector
2. Build [][]float64 of normalized unit vectors (O(N·D))
3. For i in [0, N):
     For j in [i+1, N):
       sim = dot(normalizedVecs[i], normalizedVecs[j])
       if sim >= threshold: collect pair
4. Sort collected pairs by similarity desc, return top limit
```

**Complexity**: O(N·D) normalization + O(N²·D/2) comparisons — the constant factor improvement is 2x from the triangle and the per-comparison savings from avoiding sqrt.

**Performance bounds:**
- N=1000, D=768: ~384M ops → ~0.5–1s in pure Go (acceptable)
- N=5000, D=768: ~9.6B ops → ~10–20s (borderline)
- N=10000: ~38B ops → slow; guard with a configurable `max_embeddings` cap (default 5000, documented in response)

If N exceeds cap, return a 200 with a meta warning field `"truncated": true`. No fallback path — document the limit clearly.

### Loading Embeddings Filtered by Repository

The existing `EmbeddingStore` (both SQLite and VectorChord implementations) embeds `database.Repository[search.Embedding, SQLiteEmbeddingModel]`, which exposes `Find(ctx, ...Option)`. However, filtering by repository requires joining through enrichment associations (snippet_id → enrichment_id → association → repo_id).

We add a new method `FindAll(ctx, filters search.Filters) ([]search.Embedding, error)` to the `search.EmbeddingStore` interface. The implementations:
- **SQLite**: calls `loadVectors()` (already does filter-aware DB query), converts to `[]search.Embedding`
- **VectorChord**: raw SQL query with a JOIN on enrichment associations filtered by `source_repos`

The existing `database.ApplySearchFilters()` already handles the `source_repos` join, so both implementations can reuse it.

## New Files

| File | Purpose |
|---|---|
| `application/service/duplicates.go` | `DuplicateSearch` service + `FindDuplicates()` |
| `application/service/duplicates_test.go` | Unit + integration tests |
| `infrastructure/api/v1/dto/duplicates.go` | Request/response DTOs |
| `test/e2e/duplicates_test.go` | E2E tests |

## Modified Files

| File | Change |
|---|---|
| `domain/search/store.go` | Add `FindAll(ctx, filters) ([]search.Embedding, error)` to `EmbeddingStore` interface |
| `infrastructure/persistence/embedding_store_sqlite.go` | Implement `FindAll()` |
| `infrastructure/persistence/embedding_store_vectorchord.go` | Implement `FindAll()` |
| `infrastructure/api/v1/search.go` | Add `Duplicates` handler + route `POST /duplicates` |
| `internal/mcp/catalog.go` | Add `kodit_find_duplicates` tool definition |
| `internal/mcp/server.go` | Add `DuplicateFinder` interface + `handleFindDuplicates()` handler |
| `kodit.go` | Wire `DuplicateSearch` service into `Client` |

## DuplicateSearch Service Interface

```go
// application/service/duplicates.go

type DuplicatePair struct {
    SnippetA   enrichment.Enrichment
    SnippetB   enrichment.Enrichment
    Similarity float64
}

type DuplicateSearch struct {
    codeVectorStore search.EmbeddingStore
    enrichmentStore enrichment.EnrichmentStore
    logger          zerolog.Logger
}

// FindDuplicates returns snippet pairs whose code embeddings exceed the threshold.
// If the store is nil or has no embeddings, returns empty slice (no error).
// If N embeddings exceeds maxEmbeddings (5000), truncates with a logged warning.
func (s *DuplicateSearch) FindDuplicates(
    ctx context.Context,
    repoIDs []int64,
    threshold float64,
    limit int,
) ([]DuplicatePair, bool, error)  // bool = truncated
```

## API DTO

```go
// infrastructure/api/v1/dto/duplicates.go

type DuplicateSearchAttributes struct {
    RepositoryIDs []int64  `json:"repository_ids"`
    Threshold     *float64 `json:"threshold,omitempty"` // default 0.90
    Limit         *int     `json:"limit,omitempty"`      // default 50
}

type DuplicateSearchRequest struct {
    Data struct {
        Type       string                    `json:"type"`
        Attributes DuplicateSearchAttributes `json:"attributes"`
    } `json:"data"`
}

type DuplicateSnippetSchema struct {
    ID       string `json:"id"`
    Content  string `json:"content"`
    Language string `json:"language"`
    File     string `json:"file,omitempty"`
}

type DuplicatePairAttributes struct {
    Similarity float64                `json:"similarity"`
    SnippetA   DuplicateSnippetSchema `json:"snippet_a"`
    SnippetB   DuplicateSnippetSchema `json:"snippet_b"`
}

type DuplicatePairData struct {
    Type       string                  `json:"type"`
    Attributes DuplicatePairAttributes `json:"attributes"`
}

type DuplicatesResponse struct {
    Data []DuplicatePairData `json:"data"`
    Meta *DuplicatesMeta     `json:"meta,omitempty"`
}

type DuplicatesMeta struct {
    Truncated bool `json:"truncated,omitempty"`
}
```

## Key Design Decisions

1. **No new embedding methods on the domain service** — the `FindAll()` is on the store interface, not `domain/service/EmbeddingService`. This avoids over-engineering; we only need raw vectors, not the full embedding service pipeline.

2. **Code embeddings only** — text embeddings index summaries/docs, not code. Duplicate detection applies to code. `textVectorStore` is intentionally excluded.

3. **Triangle scan in service, not SQL** — doing all-pairs in SQL (self JOIN) is database-vendor-specific and hard to threshold efficiently. The service layer loads vectors and computes in Go, matching the existing SQLite approach.

4. **max_embeddings guard = 5000** — prevents timeouts. Exposed as a constant in the service. The `truncated: true` meta field signals the client.

5. **Test with stub embedder** — the integration tests seed the SQLite code embedding table directly (same pattern as `SeedBM25` in `helpers_test.go`) to avoid running the real embedder.

## Codebase Patterns to Follow

- **Error handling**: Return `fmt.Errorf("context: %w", err)`, log only in handler
- **Options pattern**: Use `search.NewFilters(search.WithSourceRepos(repoIDs))` for repo filtering
- **Store access**: Use `database.ApplySearchFilters()` for filter-aware DB queries
- **Tests**: `testdb.New(t)` for in-memory SQLite; `gomock` for mocks; no testify
- **Build**: Always `make test`, never `go test` directly
- **No panics**: Return errors everywhere

## Manual Test Plan

With `make dev` running at `localhost:8080`:

```bash
# 1. List repositories to get IDs
curl localhost:8080/api/v1/repositories | jq .

# 2. Find duplicates with default threshold
curl -X POST localhost:8080/api/v1/search/duplicates \
  -H 'Content-Type: application/json' \
  -d '{"data":{"type":"duplicate_search","attributes":{"repository_ids":[1]}}}'

# 3. Find duplicates with strict threshold
curl -X POST localhost:8080/api/v1/search/duplicates \
  -H 'Content-Type: application/json' \
  -d '{"data":{"type":"duplicate_search","attributes":{"repository_ids":[1],"threshold":0.98}}}'

# 4. Validation error: missing repo IDs
curl -X POST localhost:8080/api/v1/search/duplicates \
  -H 'Content-Type: application/json' \
  -d '{"data":{"type":"duplicate_search","attributes":{"threshold":0.9}}}'
# Expect: 400 with validation error

# 5. Validation error: bad threshold
curl -X POST localhost:8080/api/v1/search/duplicates \
  -H 'Content-Type: application/json' \
  -d '{"data":{"type":"duplicate_search","attributes":{"repository_ids":[1],"threshold":1.5}}}'
# Expect: 400 with validation error
```
