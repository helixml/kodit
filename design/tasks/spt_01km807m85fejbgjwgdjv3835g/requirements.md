# Requirements: Duplicate Chunk Detection API & MCP Tool

## User Stories

**As a developer**, I want to call a POST endpoint to find semantically duplicated code snippets across one or more repositories, so I can identify redundant or copy-pasted code.

**As an MCP client**, I want a `kodit_find_duplicates` tool that finds similar chunks in a repository, so I can surface duplicate code to an LLM-assisted review.

## Acceptance Criteria

### API Endpoint `POST /api/v1/search/duplicates`

1. Request body (JSON:API style, like existing search endpoints):
   ```json
   {
     "data": {
       "type": "duplicate_search",
       "attributes": {
         "repository_ids": [1],
         "threshold": 0.95,
         "limit": 50
       }
     }
   }
   ```
   - `repository_ids`: required, one or more int64 repo IDs
   - `threshold`: optional float, default 0.90, range (0, 1]
   - `limit`: optional int, default 50, max 500

2. Response (pairs sorted by similarity descending):
   ```json
   {
     "data": [
       {
         "type": "duplicate_pair",
         "attributes": {
           "similarity": 0.97,
           "snippet_a": { "id": "42", "content": "...", "language": "go", "file": "..." },
           "snippet_b": { "id": "99", "content": "...", "language": "go", "file": "..." }
         }
       }
     ]
   }
   ```

3. Validation errors (400):
   - Missing/empty `repository_ids`
   - `threshold` outside (0, 1]
   - `limit` < 1
   - Invalid/non-existent repository ID

4. Returns 200 with empty `data: []` when no embeddings exist or no pairs exceed threshold.

5. Returns 200 with empty `data: []` if embeddings store is not configured.

### MCP Tool `kodit_find_duplicates`

1. Parameters:
   - `repo_url` (required): repository URL
   - `threshold` (optional, number): similarity threshold (default 0.90)
   - `limit` (optional, number): max pairs to return (default 20)

2. Returns JSON array of duplicate pairs matching the format above.

### Application Service `service.FindDuplicates`

1. Accepts: `ctx`, repo IDs `[]int64`, threshold `float64`, limit `int`
2. Loads all code embeddings for the given repos
3. Computes pairwise cosine similarity (see Design for algorithm)
4. Returns pairs with similarity ≥ threshold, sorted descending, capped at limit
5. Returns empty slice (not error) when no embeddings exist

### Tests

1. **Unit tests** for the pairwise similarity algorithm (edge cases: zero vectors, identical vectors, empty input, single vector, threshold boundary)
2. **E2E test** in `test/e2e/` verifying:
   - Empty result with no embeddings
   - Missing `repository_ids` → 400
   - Invalid threshold → 400
   - Pairs returned when seeded embeddings exceed threshold
3. **Application service test** (`application/service/duplicates_test.go`) covering:
   - No embeddings → empty result
   - Two identical snippets → similarity 1.0, returned
   - Two dissimilar snippets → not returned
   - Threshold boundary: pair exactly at threshold is included
   - Limit cap: only top-N pairs returned
   - Multiple repos: finds cross-repo duplicates
