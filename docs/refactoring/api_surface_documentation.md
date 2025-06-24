# Current API Surface Documentation

## IndexingApplicationService

### Methods

1. **create_index(source_id: int) -> IndexView**
   - Creates a new index for a source
   - Validates source exists
   - Commits transaction

2. **list_indexes() -> list[IndexView]**
   - Lists all available indexes
   - Logs telemetry data

3. **run_index(index_id: int, progress_callback: ProgressCallback | None = None) -> None**
   - Orchestrates the complete indexing workflow:
     - Deletes old snippets
     - Creates new snippets (delegates to SnippetApplicationService)
     - Creates BM25 index
     - Creates code embeddings
     - Enriches snippets
     - Creates text embeddings
     - Updates index timestamp
   - Multiple transaction commits

4. **search(request: MultiSearchRequest) -> list[MultiSearchResult]**
   - Performs unified search across all indexes
   - Supports keyword, code, and text queries
   - Applies filters by delegating to SnippetApplicationService
   - Uses fusion for ranking

### Private Methods

- _create_bm25_index()
- _create_code_embeddings()
- _enrich_snippets()
- _create_text_embeddings()

## SnippetApplicationService

### Methods

1. **extract_snippets_from_file(command: ExtractSnippetsCommand) -> list[Snippet]**
   - Extracts snippets from a single file
   - Returns in-memory Snippet entities (not persisted)

2. **create_snippets_for_index(command: CreateIndexSnippetsCommand, progress_callback: ProgressCallback | None = None) -> None**
   - Creates snippets for all files in an index
   - Persists snippets to database
   - Single transaction commit

3. **list_snippets(command: ListSnippetsCommand) -> list[SnippetListItem]**
   - Lists snippets with optional filtering by file path or source URI

4. **search(request: MultiSearchRequest) -> list[SnippetListItem]**
   - Searches snippets with filters only
   - Used by IndexingApplicationService for pre-filtering

### Private Methods

- _should_process_file()
- _extract_snippets_from_file()

## Key Observations

1. **Circular Dependency**: IndexingApplicationService depends on SnippetApplicationService
2. **Transaction Boundaries**: Multiple commits across services
3. **Inconsistent Return Types**: search() returns different types in each service
4. **Split Responsibilities**: Indexing workflow is split between services
5. **Progress Reporting**: Duplicated across both services
