# Design Document: CLI Remote Connection Refactoring

## Executive Summary

This design document outlines the architectural refactoring of the Kodit CLI to enable remote CLI clients to communicate with a Kodit instance running as a server. The refactoring adds HTTP/REST API client capabilities to the CLI while maintaining the existing local mode for development and single-user scenarios.

## Goals and Objectives

### Primary Goals

1. **Enable remote CLI access**: Allow CLI clients to connect to and manage remote Kodit server instances
2. **Maintain backward compatibility**: Preserve local mode for development and single-user scenarios
3. **Follow DDD principles**: Place infrastructure concerns in appropriate layers
4. **Leverage existing API**: Use current REST endpoints without requiring server changes

### Non-Goals

- Rewriting the MCP server functionality
- Changing the existing API contract
- Modifying the domain layer architecture

## Current Architecture Analysis

### Existing CLI Structure

The current CLI implementation (`src/kodit/cli.py`) has the following characteristics:

1. **Direct Database Access**: Commands use `@with_session` decorator to get database sessions
2. **Local Service Instantiation**: Creates application services directly within commands
3. **Synchronous Operations**: All operations run in the CLI process
4. **Commands**:
   - `index`: Create, list, sync indexes
   - `search code/keyword/text/hybrid`: Search operations
   - `show snippets`: Display snippets
   - `serve`: Start HTTP/SSE server
   - `stdio`: Start MCP server
   - `version`: Show version

### Existing API Infrastructure

The server currently provides these REST endpoints:

- `GET /api/v1/indexes`: List all indexes
- `POST /api/v1/indexes`: Create new index
- `GET /api/v1/indexes/{id}`: Get index details
- `DELETE /api/v1/indexes/{id}`: Delete index
- `POST /api/v1/search`: Search snippets

**CLI Commands Not Available via API**:
The following CLI commands cannot be mapped to existing API endpoints and will show "Not implemented in remote mode" warnings:

- `index --sync`: Sync existing indexes with remotes
- `show snippets`: Display snippets with filtering

## Proposed Architecture

### High-Level Design

```
┌─────────────┐           ┌──────────────┐           ┌────────────┐
│  Kodit CLI  │  HTTP/S   │ Kodit Server │           │  Database  │
│   (Client)  │◄─────────►│   (FastAPI)  │◄─────────►│(PostgreSQL)│
└─────────────┘           └──────────────┘           └────────────┘
       │                          │
       │                          │
  Configuration              Domain Services
   (local/remote)             (unchanged)
```

### CLI Command Mapping to API

| CLI Command | Local Mode | Remote Mode | API Endpoint |
|------------|------------|-------------|--------------|
| `index` (list) | ✅ Direct DB | ✅ API | `GET /api/v1/indexes` |
| `index <source>` | ✅ Direct DB | ✅ API | `POST /api/v1/indexes` |
| `index --sync` | ✅ Direct DB | ❌ Not available | N/A - No endpoint |
| `index --auto-index` | ✅ Direct DB | ❌ Not available | N/A - Server handles |
| `search code` | ✅ Direct DB | ✅ API | `POST /api/v1/search` |
| `search keyword` | ✅ Direct DB | ✅ API | `POST /api/v1/search` |
| `search text` | ✅ Direct DB | ✅ API | `POST /api/v1/search` |
| `search hybrid` | ✅ Direct DB | ✅ API | `POST /api/v1/search` |
| `show snippets` | ✅ Direct DB | ❌ Not available | N/A - No endpoint |
| `serve` | ✅ Local only | N/A | N/A - Starts server |
| `stdio` | ✅ Local only | N/A | N/A - MCP server |
| `version` | ✅ Local | ✅ Local | N/A - CLI version |

### Component Architecture

#### 1. API Client Layer (Infrastructure)

Following DDD principles, the API client code belongs in the infrastructure layer since it deals with external communication:

```python
kodit/infrastructure/api/client/
├── __init__.py
├── base.py          # Base HTTP client with auth, retry logic
├── index_client.py  # Index operations API client
├── search_client.py # Search operations API client
└── exceptions.py    # Client-specific exceptions
```

#### 2. Connection Mode Detection

The CLI will automatically detect the connection mode based on configuration:

**Remote Mode** (when server URL is configured):

- Activated when `REMOTE_SERVER_URL` is set or `--server` flag is provided
- Connects to a remote Kodit server via HTTP/HTTPS
- Uses API key authentication
- All operations go through REST API

**Local Mode** (default when no server URL):

- Direct database connection (current behavior)
- Used when no remote server is configured
- Useful for development and single-user scenarios

#### 3. Configuration Schema

Enhanced configuration to support remote connections using the existing nested configuration pattern:

```python
class RemoteConfig(BaseModel):
    """Configuration for remote server connection."""
    
    server_url: str | None = Field(
        default=None,
        description="Remote Kodit server URL"
    )
    api_key: str | None = Field(
        default=None,
        description="API key for authentication"
    )
    timeout: float = Field(
        default=30.0,
        description="Request timeout in seconds"
    )
    max_retries: int = Field(
        default=3,
        description="Maximum retry attempts"
    )
    verify_ssl: bool = Field(
        default=True,
        description="Verify SSL certificates"
    )

class AppContext(BaseSettings):
    # ... existing fields ...
    
    remote: RemoteConfig = Field(
        default_factory=RemoteConfig,
        description="Remote server configuration"
    )
    
    @property
    def is_remote(self) -> bool:
        """Check if running in remote mode."""
        return self.remote.server_url is not None
```

## Implementation Plan

### Phase 1: API Client Implementation

#### 1.1 Base HTTP Client

```python
# src/kodit/infrastructure/api/client/base.py
import httpx
from typing import Any, Dict, Optional
from kodit.infrastructure.api.client.exceptions import KoditAPIError, AuthenticationError

class BaseAPIClient:
    def __init__(
        self,
        base_url: str,
        api_key: Optional[str] = None,
        timeout: float = 30.0,
        max_retries: int = 3,
        verify_ssl: bool = True
    ):
        self.base_url = base_url.rstrip('/')
        self.api_key = api_key
        self.timeout = timeout
        self.max_retries = max_retries
        self.verify_ssl = verify_ssl
        self._client = self._create_client()
    
    def _create_client(self) -> httpx.AsyncClient:
        headers = {}
        if self.api_key:
            headers["X-API-Key"] = self.api_key
        
        return httpx.AsyncClient(
            base_url=self.base_url,
            headers=headers,
            timeout=httpx.Timeout(self.timeout),
            verify=self.verify_ssl,
            follow_redirects=True
        )
    
    async def _request(
        self,
        method: str,
        path: str,
        **kwargs
    ) -> httpx.Response:
        """Make HTTP request with retry logic."""
        url = f"{self.base_url}{path}"
        
        for attempt in range(self.max_retries):
            try:
                response = await self._client.request(method, url, **kwargs)
                response.raise_for_status()
                return response
            except httpx.HTTPStatusError as e:
                if e.response.status_code == 401:
                    raise AuthenticationError("Invalid API key")
                elif e.response.status_code >= 500 and attempt < self.max_retries - 1:
                    await asyncio.sleep(2 ** attempt)  # Exponential backoff
                    continue
                raise KoditAPIError(f"API request failed: {e}")
            except httpx.RequestError as e:
                if attempt < self.max_retries - 1:
                    await asyncio.sleep(2 ** attempt)
                    continue
                raise KoditAPIError(f"Connection error: {e}")
        
        raise KoditAPIError(f"Max retries ({self.max_retries}) exceeded")
    
    async def close(self):
        await self._client.aclose()
```

#### 1.2 Index Operations Client

```python
# src/kodit/infrastructure/api/client/index_client.py
from typing import List, Optional
from kodit.infrastructure.api.client.base import BaseAPIClient
from kodit.infrastructure.api.v1.schemas.index import (
    IndexData, IndexCreateRequest, IndexListResponse
)

class IndexClient(BaseAPIClient):
    async def list_indexes(self) -> List[IndexData]:
        """List all indexes."""
        response = await self._request("GET", "/api/v1/indexes")
        data = IndexListResponse.model_validate_json(response.text)
        return data.data
    
    async def create_index(self, uri: str) -> IndexData:
        """Create a new index."""
        request = IndexCreateRequest(
            data=IndexData(
                type="index",
                attributes={"uri": uri}
            )
        )
        response = await self._request(
            "POST",
            "/api/v1/indexes",
            json=request.model_dump()
        )
        result = response.json()
        return IndexData.model_validate(result["data"])
    
    async def get_index(self, index_id: str) -> Optional[IndexData]:
        """Get index by ID."""
        try:
            response = await self._request("GET", f"/api/v1/indexes/{index_id}")
            result = response.json()
            return IndexData.model_validate(result["data"])
        except KoditAPIError as e:
            if "404" in str(e):
                return None
            raise
    
    async def delete_index(self, index_id: str) -> None:
        """Delete an index."""
        await self._request("DELETE", f"/api/v1/indexes/{index_id}")
```

#### 1.3 Search Operations Client

```python
# src/kodit/infrastructure/api/client/search_client.py
from typing import List, Optional
from kodit.infrastructure.api.client.base import BaseAPIClient
from kodit.infrastructure.api.v1.schemas.search import (
    SearchRequest, SearchResponse, SnippetData
)

class SearchClient(BaseAPIClient):
    async def search(
        self,
        keywords: Optional[List[str]] = None,
        code_query: Optional[str] = None,
        text_query: Optional[str] = None,
        limit: int = 10,
        languages: Optional[List[str]] = None,
        authors: Optional[List[str]] = None,
        start_date: Optional[str] = None,
        end_date: Optional[str] = None,
        sources: Optional[List[str]] = None,
        file_patterns: Optional[List[str]] = None
    ) -> List[SnippetData]:
        """Search for code snippets."""
        request = SearchRequest(
            data={
                "type": "search",
                "attributes": {
                    "keywords": keywords,
                    "code": code_query,
                    "text": text_query
                }
            },
            limit=limit,
            languages=languages,
            authors=authors,
            start_date=start_date,
            end_date=end_date,
            sources=sources,
            file_patterns=file_patterns
        )
        
        response = await self._request(
            "POST",
            "/api/v1/search",
            json=request.model_dump(exclude_none=True)
        )
        
        result = SearchResponse.model_validate_json(response.text)
        return result.data
```

### Phase 2: CLI Command Refactoring

#### 2.1 Mode-Aware Command Decorator

```python
# src/kodit/cli_utils.py
from functools import wraps
from typing import Callable

def with_client(f: Callable) -> Callable:
    """Decorator that provides appropriate client based on configuration."""
    @wraps(f)
    async def wrapper(*args, **kwargs):
        ctx = click.get_current_context()
        app_context: AppContext = ctx.obj
        
        # Auto-detect mode based on remote.server_url presence
        if not app_context.is_remote:
            # Use existing local implementation
            return await f(*args, **kwargs)
        else:
            # Inject API clients for remote mode
            clients = {
                "index_client": IndexClient(
                    base_url=app_context.remote.server_url,
                    api_key=app_context.remote.api_key,
                    timeout=app_context.remote.timeout,
                    max_retries=app_context.remote.max_retries,
                    verify_ssl=app_context.remote.verify_ssl
                ),
                "search_client": SearchClient(
                    base_url=app_context.remote.server_url,
                    api_key=app_context.remote.api_key,
                    timeout=app_context.remote.timeout,
                    max_retries=app_context.remote.max_retries,
                    verify_ssl=app_context.remote.verify_ssl
                )
            }
            
            try:
                return await f(*args, clients=clients, **kwargs)
            finally:
                for client in clients.values():
                    await client.close()
    
    return wrapper
```

#### 2.2 Refactored Index Command

```python
# src/kodit/cli.py (refactored index command)

@cli.command()
@click.argument("sources", nargs=-1)
@click.option("--auto-index", is_flag=True)
@click.option("--sync", is_flag=True)
@click.option("--server", help="Remote server URL (overrides env)")
@click.option("--api-key", help="API key for remote server (overrides env)")
@with_app_context
@with_client
async def index(
    app_context: AppContext,
    sources: list[str],
    auto_index: bool,
    sync: bool,
    server: str = None,
    api_key: str = None,
    clients: dict = None
):
    """List indexes, index data sources, or sync existing indexes."""
    
    # Override remote configuration if provided via CLI
    if server:
        app_context.remote.server_url = server
    if api_key:
        app_context.remote.api_key = api_key
    
    if not app_context.is_remote:
        # Use existing local implementation
        await _index_local(app_context, sources, auto_index, sync)
    else:
        # Use remote API client
        await _index_remote(clients, sources, auto_index, sync)

async def _index_remote(
    clients: dict,
    sources: list[str],
    auto_index: bool,
    sync: bool
):
    """Handle remote index operations."""
    index_client = clients["index_client"]
    
    if sync:
        # Sync operation not available in remote mode
        click.echo("⚠️  Warning: Index sync is not implemented in remote mode")
        click.echo("Please use the server's auto-sync functionality or sync locally")
        return
    
    if not sources:
        # List indexes
        indexes = await index_client.list_indexes()
        _display_indexes(indexes)
        return
    
    # Create new indexes
    for source in sources:
        click.echo(f"Creating index for: {source}")
        index = await index_client.create_index(source)
        click.echo(f"✓ Index created with ID: {index.id}")
```

### Phase 3: Handle Unsupported Operations

For CLI commands that don't have corresponding API endpoints, provide clear warnings:

```python
# src/kodit/cli.py (handling unsupported operations)

async def _show_snippets_remote(clients: dict, by_path: str, by_source: str):
    """Handle remote snippet listing - not supported."""
    click.echo("⚠️  Warning: 'show snippets' is not implemented in remote mode")
    click.echo("This functionality is only available when connected directly to the database")
    click.echo("Use 'kodit search' commands instead for remote snippet retrieval")
```

## Migration Strategy

### Phase 1: Preparation

1. Implement API client layer with base HTTP client
2. Create specialized clients for indexes and search
3. Update configuration schema for server URL and API key
4. Write comprehensive tests for clients

### Phase 2: CLI Refactoring

1. Add mode detection based on server URL configuration
2. Refactor commands to use API clients when in remote mode
3. Add warning messages for unsupported remote operations
4. Ensure backward compatibility with local mode

### Phase 3: Testing & Documentation

1. End-to-end testing of remote mode with existing API
2. Test error handling and retry logic
3. Update documentation with remote connection examples
4. Create migration guide for users

## Testing Strategy

### Unit Tests

- Test API clients in isolation with mocked HTTP responses
- Test command logic with both local and remote modes
- Test configuration parsing and validation

### Integration Tests

- Test end-to-end workflows with real server
- Test authentication and authorization
- Test error handling and retries
- Test progress tracking and streaming

### Performance Tests

- Measure latency of remote operations vs local
- Test concurrent operations
- Test large-scale indexing and search

## Security Considerations

### Authentication

- API key-based authentication for simplicity
- Optional OAuth2/JWT for enterprise deployments
- Secure storage of API keys in local configuration

### Authorization

- Role-based access control (future enhancement)
- Per-index permissions (future enhancement)
- Audit logging of all operations

### Network Security

- HTTPS enforcement for production
- Certificate validation
- Request signing for critical operations

## Monitoring and Observability

### Metrics

- API request/response times
- Error rates by endpoint
- Active connections
- Queue depths for async operations

### Logging

- Structured logging with correlation IDs
- Request/response logging (sanitized)
- Error tracking and alerting

### Health Checks

- `/healthz` endpoint for server health
- Database connectivity checks
- Background worker status

## Configuration Examples

### Remote Mode (Automatic when server URL is configured)

```bash
# .env file
REMOTE_SERVER_URL=https://kodit.example.com
REMOTE_API_KEY=sk-xxxxxxxxxxxxx
REMOTE_TIMEOUT=60.0
REMOTE_MAX_RETRIES=5

# CLI usage (automatically uses remote mode)
kodit index https://github.com/example/repo
kodit search code "def calculate"
```

### Local Mode (Default when no server URL)

```bash
# .env file
DB_URL=postgresql://localhost/kodit
# No REMOTE_SERVER_URL set = local mode

# CLI usage (automatically uses local mode)
kodit index /path/to/repo
kodit search keyword "async" "await"
```

### Flexible Usage

```bash
# Connect to staging server via CLI flag
kodit --server https://staging.kodit.example.com index list

# Use environment variable for production
export REMOTE_SERVER_URL=https://prod.kodit.example.com
kodit search text "machine learning"

# Override server on the fly
kodit --server https://dev.kodit.example.com --api-key dev-key-123 index sync
```

## Open Questions and Future Considerations

1. **WebSocket Support**: Should we add WebSocket support for real-time updates?
2. **Batch Operations**: How to handle bulk operations efficiently?
3. **Caching Strategy**: Should the CLI cache responses for better performance?
4. **Offline Mode**: Should we support offline operations with sync?
5. **Multi-tenancy**: How to support multiple organizations/projects?
6. **Rate Limiting**: How to handle rate limiting gracefully?
7. **Versioning**: How to handle API version compatibility?

## Conclusion

This refactoring transforms the Kodit CLI from a monolithic database-coupled tool into a flexible client that can connect to both local and remote Kodit instances. The design:

1. **Maintains simplicity**: Uses existing API endpoints without requiring server changes
2. **Provides clear feedback**: Shows warning messages for operations not available in remote mode
3. **Enables flexible deployment**: Supports both local development and remote server connections
4. **Preserves backward compatibility**: Local mode continues to work exactly as before

The implementation focuses on practicality - mapping existing CLI functionality to
available REST API endpoints and clearly communicating limitations when operations
aren't supported remotely. This approach allows for immediate deployment without
server-side changes while providing a foundation for future enhancements.
