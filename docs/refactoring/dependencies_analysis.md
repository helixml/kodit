# Dependencies Analysis

## Usage Locations

### IndexingApplicationService

1. **CLI** (`src/kodit/cli.py`):
   - `index` command - creates and runs indexes
   - `search code` command - semantic code search
   - `search keyword` command - keyword search
   - `search text` command - semantic text search
   - `search hybrid` command - combined search

2. **MCP Server** (`src/kodit/mcp.py`):
   - Search functionality exposed via MCP protocol

3. **Factory** (`src/kodit/infrastructure/indexing/indexing_factory.py`):
   - `create_indexing_application_service()` - creates service with all dependencies

### SnippetApplicationService

1. **CLI** (`src/kodit/cli.py`):
   - `show snippets` command - lists snippets with filtering
   - Created as dependency for IndexingApplicationService

2. **MCP Server** (`src/kodit/mcp.py`):
   - Created but not directly used (only as dependency)

3. **Factory** (`src/kodit/infrastructure/indexing/indexing_factory.py`):
   - `create_snippet_application_service()` - creates service
   - Passed as dependency to IndexingApplicationService

4. **IndexingApplicationService** (circular dependency):
   - Used in `run_index()` for snippet creation
   - Used in `search()` for pre-filtering

## Dependency Graph

```
CLI Commands
├── index command
│   ├── create_indexing_application_service()
│   │   └── create_snippet_application_service()
│   └── Uses: create_index(), run_index()
├── search commands (code/keyword/text/hybrid)
│   ├── create_indexing_application_service()
│   │   └── create_snippet_application_service()
│   └── Uses: search()
└── show snippets command
    ├── create_snippet_application_service()
    └── Uses: list_snippets()

MCP Server
├── create_indexing_application_service()
│   └── create_snippet_application_service()
└── Uses: search()
```

## Key Findings

1. **Primary Entry Points**:
   - CLI commands
   - MCP server

2. **Service Creation Pattern**:
   - Always create SnippetApplicationService first
   - Pass it as dependency to IndexingApplicationService
   - Factories handle all dependency injection

3. **Usage Patterns**:
   - IndexingApplicationService is the main service for most operations
   - SnippetApplicationService is only used directly for `show snippets`
   - All search operations go through IndexingApplicationService

4. **Refactoring Impact**:
   - Need to update both CLI and MCP server
   - Factory methods need to be revised
   - Tests need to be updated
