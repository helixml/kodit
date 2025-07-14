# Product Requirement Prompt: REST API for Index Management and Search

## Executive Summary

### Goal

Implement a comprehensive REST API for managing code indexes and searching code snippets, providing programmatic access to Kodit's indexing and search capabilities with secure token-based authentication.

### Why

- **Business Value:** Enable integration with external tools, CI/CD pipelines, and custom workflows
- **User Benefits:** Programmatic access to index management and search without MCP client requirement
- **Technical Benefits:** Standardized HTTP interface, easier monitoring, and broader tool compatibility
- **Strategic Alignment:** Complements existing MCP server with industry-standard REST patterns

### What

**Core Functionality:**

- Index management endpoints (CRUD operations) under `/api/v1/indexes`
- Asynchronous index creation with status tracking -- don't create the status tracking
  API yet. Leave that for a future requirement.
- Search API under `/api/v1/search` with full MCP/CLI feature parity
- Token-based authentication with auto-generation and env var support

## Requirements Analysis

### User Requirements Summary

- Versioned API structure for future compatibility
- Simple token authentication with automatic setup
- Asynchronous indexing to handle large codebases
- Feature parity with existing MCP search functionality
- No pagination initially (can be added later)

### Technical Requirements Summary  

- Integrate with existing FastAPI application
- Use repository pattern from domain layer
- Implement authentication middleware
- Support all existing search filters
- Maintain clean architecture principles

### Key Decisions Made

- **API Versioning**: Use `/api/v1/` prefix for future-proofing
- **Authentication**: Token-based with CSV env var and auto-generation
- **Async Indexing**: Return immediately without status tracking
- **Search Parity**: Full compatibility with MCP tool filters
- **No Pagination**: Simplify initial implementation

## Architecture & Implementation

### Technical Overview

- **Architecture Pattern:** Clean Architecture with API layer on top of existing domain
- **Technology Stack:** FastAPI, SQLAlchemy (async), Pydantic for validation
- **Integration Points:** Domain services, repository pattern, existing MCP search
- **Data Flow:** API → Application Services → Domain Services → Infrastructure

### Directory Structure

**Current Relevant Structure:**

```
src/kodit/
├── app.py                    # Main FastAPI application
├── config.py                 # Configuration with env vars
├── domain/
│   ├── entities.py          # Domain entities
│   ├── protocols.py         # Repository interfaces
│   └── value_objects.py     # Request/response objects
├── application/
│   └── factories/           # Service factories
├── infrastructure/
│   └── sqlalchemy/          # Database implementation
└── mcp.py                   # MCP server implementation
```

**Proposed Changes:**

```
src/kodit/
├── infrastructure/
│   ├── api/                 # NEW: API infrastructure layer
│   │   ├── __init__.py
│   │   ├── v1/              # NEW: Version 1 API
│   │   │   ├── __init__.py
│   │   │   ├── routers/     # NEW: API routers
│   │   │   │   ├── __init__.py
│   │   │   │   ├── indexes.py  # Index management endpoints
│   │   │   │   └── search.py   # Search and snippet endpoints
│   │   │   ├── schemas/     # NEW: API request/response schemas
│   │   │   │   ├── __init__.py
│   │   │   │   ├── index.py    # Index-related schemas
│   │   │   │   └── search.py   # Search-related schemas
│   │   │   └── dependencies.py  # NEW: Shared dependencies
│   │   ├── middleware/      # NEW: API middleware
│   │   │   ├── __init__.py
│   │   │   └── auth.py     # Token authentication
│   │   └── utils/           # NEW: API utilities
│   │       ├── __init__.py
│   │       └── tokens.py   # Token generation/management
│   └── sqlalchemy/          # Database implementation
├── app.py                   # MODIFIED: Include API routers
└── config.py                # MODIFIED: Add API token config
```

### Files to Reference

- **`src/kodit/app.py`** (existing) - Main app structure and middleware patterns
- **`src/kodit/mcp.py`** (existing) - Search implementation to replicate in REST
- **`src/kodit/domain/protocols.py`** (existing) - Repository interfaces to use
- **`src/kodit/infrastructure/sqlalchemy/index_repository.py`** (existing) - Repository implementation
- **FastAPI documentation** (external) - Authentication and router patterns

### Implementation Specifications

#### Authentication Middleware

**File: `src/kodit/infrastructure/api/middleware/auth.py`**
Purpose: Token-based authentication with auto-generation

```python
import secrets
import os
from pathlib import Path
from typing import Optional, Set
from fastapi import Request, HTTPException
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from starlette.middleware.base import BaseHTTPMiddleware

class TokenAuthMiddleware(BaseHTTPMiddleware):
    def __init__(self, app, tokens: Set[str], data_dir: Path):
        super().__init__(app)
        self.valid_tokens = tokens
        self.token_file = self.data_dir / 'api_token.txt'
        
        # Auto-generate token if none provided
        if not self.valid_tokens:
            # Check if api_token.txt exists
            if token_file.exists():
                # Read token from file...
            else:
                token = self._generate_and_save_token()
                print(f"\n🔐 Generated API token: {token}")
                print(f"Token saved to: {}\n")
            self.valid_tokens.add(token)

    def _generate_and_save_token(self) -> str:
        token = f"kodit_{secrets.token_urlsafe(32)}"
        token_file = self.data_dir / "api_token.txt"
        token_file.parent.mkdir(parents=True, exist_ok=True)
        token_file.write_text(token)
        return token
    
    async def dispatch(self, request: Request, call_next):
        # Skip auth for health endpoints
        if request.url.path in ["/", "/healthz"]:
            return await call_next(request)
        
        # Only protect /api/* endpoints
        if request.url.path.startswith("/api/"):
            auth_header = request.headers.get("Authorization")
            if not auth_header or not auth_header.startswith("Bearer "):
                raise HTTPException(status_code=401, detail="Missing or invalid authorization header")
            
            token = auth_header.split(" ", 1)[1]
            if token not in self.valid_tokens:
                raise HTTPException(status_code=401, detail="Invalid token")
        
        return await call_next(request)
```

#### Index Management Router

**File: `src/kodit/infrastructure/api/v1/routers/indexes.py`**
Purpose: CRUD operations for indexes using the application service

```python
from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks
from typing import List

from kodit.infrastructure.api.v1.schemas.index import (
    IndexResponse, IndexCreateRequest, IndexListResponse
)
from kodit.infrastructure.api.v1.dependencies import get_indexing_app_service, get_index_repository
from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService
)
from kodit.domain.protocols import IndexRepository

router = APIRouter(prefix="/api/v1/indexes", tags=["indexes"])

@router.get("", response_model=IndexListResponse)
async def list_indexes(
    repo: IndexRepository = Depends(get_index_repository)
) -> IndexListResponse:
    """List all indexes"""
    indexes = await repo.list_all()
    return {
        "data": [
            {
                "type": "index",
                "id": str(idx.id),
                "attributes": {
                    "created_at": idx.created_at,
                    "updated_at": idx.updated_at
                },
                "relationships": {
                    "source": {
                        "data": {
                            "type": "source",
                            "id": str(idx.source.id)
                        }
                    }
                }
            }
            for idx in indexes
        ]
    }

@router.post("", response_model=IndexResponse, status_code=202)
async def create_index(
    request: IndexCreateRequest,
    background_tasks: BackgroundTasks,
    app_service: CodeIndexingApplicationService = Depends(get_indexing_app_service)
) -> IndexResponse:
    """Create a new index and start async indexing"""
    
    # Create index using the application service
    index = await app_service.create_index_from_uri(request.source_path)
    
    # Start async indexing in background
    background_tasks.add_task(
        app_service.run_index,
        index
    )
    
    return {
        "data": {
            "type": "index",
            "id": str(index.id),
            "attributes": {
                "created_at": index.created_at,
                "updated_at": index.updated_at
            },
            "relationships": {
                "source": {
                    "data": {
                        "type": "source",
                        "id": str(index.source.id)
                    }
                }
            }
        }
    }

@router.get("/{index_id}", response_model=IndexResponse)
async def get_index(
    index_id: str,
    repo: IndexRepository = Depends(get_index_repository)
) -> IndexResponse:
    """Get index details"""
    index = await repo.get(index_id)
    if not index:
        raise HTTPException(status_code=404, detail="Index not found")
    
    return {
        "data": {
            "type": "index",
            "id": str(index.id),
            "attributes": {
                "created_at": index.created_at,
                "updated_at": index.updated_at
            },
            "relationships": {
                "source": {
                    "data": {
                        "type": "source",
                        "id": str(index.source.id)
                    }
                },
                "snippets": {
                    "data": [
                        {
                            "type": "snippet",
                            "id": str(snippet.id)
                        }
                        for snippet in index.snippets
                    ]
                }
            }
        }
    }

@router.delete("/{index_id}", status_code=204)
async def delete_index(
    index_id: str,
    repo: IndexRepository = Depends(get_index_repository)
) -> None:
    """Delete an index"""
    index = await repo.get(index_id)
    if not index:
        raise HTTPException(status_code=404, detail="Index not found")
    
    await repo.delete(index_id)
```

#### Search API Router

**File: `src/kodit/infrastructure/api/v1/routers/search.py`**
Purpose: Search endpoints matching MCP functionality

```python
from fastapi import APIRouter, Depends, HTTPException
from typing import Optional, List
from datetime import datetime

from kodit.infrastructure.api.v1.schemas.search import (
    SearchRequest, SearchResponse, SnippetResponse
)
from kodit.infrastructure.api.v1.dependencies import get_indexing_app_service, get_index_query_service
from kodit.application.services.code_indexing_application_service import (
    CodeIndexingApplicationService
)
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.value_objects import MultiSearchRequest, SnippetSearchFilters

router = APIRouter(tags=["search"])

@router.post("/api/v1/search", response_model=SearchResponse)
async def search_snippets(
    request: SearchRequest,
    app_service: CodeIndexingApplicationService = Depends(get_indexing_app_service)
) -> SearchResponse:
    """Search code snippets with filters matching MCP tool"""
    
    # Build queries for different search types
    keywords = []
    code_query = None
    text_query = None
    
    # Parse queries to determine search types
    for query in request.queries:
        # For now, treat all queries as keywords
        # Future enhancement: add query type hints
        keywords.append(query)
    
    # Convert API request to domain request
    domain_request = MultiSearchRequest(
        keywords=keywords,
        code_query=code_query,
        text_query=text_query,
        top_k=request.limit or 10,
        filters=SnippetSearchFilters(
            languages=request.languages,
            authors=request.authors,
            start_date=request.start_date,
            end_date=request.end_date,
            sources=request.sources,
            file_patterns=request.file_patterns
        ) if any([
            request.languages, request.authors, request.start_date,
            request.end_date, request.sources, request.file_patterns
        ]) else None
    )
    
    # Execute search using application service
    results = await app_service.search(domain_request)
    
    return {
        "data": [
            {
                "type": "snippet",
                "id": str(result.id),
                "attributes": {
                    "content": result.content,
                    "created_at": result.created_at,
                    "updated_at": result.created_at,  # Use created_at as fallback
                    "original_scores": result.original_scores,
                    "source_uri": result.source_uri,
                    "relative_path": result.relative_path,
                    "language": result.language,
                    "authors": result.authors,
                    "summary": result.summary
                }
            }
            for result in results
        ]
    }
```

## API/Endpoints Design

Following JSON:API conventions (https://jsonapi.org/), all responses include `data` wrapper with `type` and `id` fields.

### Index Management Endpoints

#### List Indexes

- **Method:** GET
- **Path:** `/api/v1/indexes`
- **Purpose:** Retrieve all indexes
- **Headers:** `Authorization: Bearer <token>`
- **Success Response:**

  ```json
  {
    "data": [
      {
        "type": "index",
        "id": "123",
        "attributes": {
          "created_at": "2025-01-14T09:00:00Z",
          "updated_at": "2025-01-14T09:00:00Z"
        },
        "relationships": {
          "source": {
            "data": {
              "type": "source",
              "id": "456"
            }
          }
        }
      }
    ]
  }
  ```

#### Create Index

- **Method:** POST
- **Path:** `/api/v1/indexes`
- **Purpose:** Create new index (async)
- **Headers:** `Authorization: Bearer <token>`
- **Request Body:**

  ```json
  {
    "data": {
      "type": "index",
      "attributes": {
        "source_uri": "https://github.com/user/repo"
      }
    }
  }
  ```

- **Success Response (202 Accepted):**

  ```json
  {
    "data": {
      "type": "index",
      "id": "123",
      "attributes": {
        "created_at": "2025-01-14T09:00:00Z",
        "updated_at": "2025-01-14T09:00:00Z"
      },
      "relationships": {
        "source": {
          "data": {
            "type": "source",
            "id": "456"
          }
        }
      }
    }
  }
  ```

#### Get Index Details

- **Method:** GET
- **Path:** `/api/v1/indexes/{id}`
- **Purpose:** Get index details
- **Headers:** `Authorization: Bearer <token>`
- **Success Response:**

  ```json
  {
    "data": {
      "type": "index",
      "id": "123",
      "attributes": {
        "created_at": "2025-01-14T09:00:00Z",
        "updated_at": "2025-01-14T09:00:00Z"
      },
      "relationships": {
        "source": {
          "data": {
            "type": "source",
            "id": "456"
          }
        },
        "snippets": {
          "data": [
            {
              "type": "snippet",
              "id": "789"
            }
          ]
        }
      }
    },
    "included": [
      {
        "type": "source",
        "id": "456",
        "attributes": {
          "created_at": "2025-01-14T09:00:00Z",
          "updated_at": "2025-01-14T09:00:00Z",
          "remote_uri": "https://github.com/user/repo",
          "cloned_path": "/tmp/kodit/abc123",
          "source_type": "git"
        }
      }
    ]
  }
  ```

#### Delete Index

- **Method:** DELETE
- **Path:** `/api/v1/indexes/{id}`
- **Purpose:** Delete an index
- **Headers:** `Authorization: Bearer <token>`
- **Success Response:** 204 No Content

### Search Endpoints

#### Search Snippets

- **Method:** POST
- **Path:** `/api/v1/search`
- **Purpose:** Search code snippets with filters
- **Headers:** `Authorization: Bearer <token>`
- **Request Body:**

  ```json
  {
    "data": {
      "type": "search",
      "attributes": {
        "keywords": ["logger implementation", "error handling"],
        "top_k": 20,
        "filters": {
          "language": "python",
          "author": "alice@example.com",
          "created_after": "2025-01-01",
          "created_before": "2025-01-14",
          "source_repo": "https://github.com/user/repo",
          "file_path": "**/src/**"
        }
      }
    }
  }
  ```

- **Success Response:**

  ```json
  {
    "data": [
      {
        "type": "snippet",
        "id": "789",
        "attributes": {
          "content": "def setup_logger(name: str):\n    logger = logging.getLogger(name)\n    ...",
          "created_at": "2025-01-10T14:30:00Z",
          "updated_at": "2025-01-10T14:30:00Z",
          "original_scores": [0.95, 0.87],
          "source_uri": "https://github.com/user/repo",
          "relative_path": "src/utils/logging.py",
          "language": "python",
          "authors": ["alice@example.com"],
          "summary": "Logger setup utility function"
        },
        "relationships": {
          "files": {
            "data": [
              {
                "type": "file",
                "id": "101"
              }
            ]
          }
        }
      }
    ],
    "included": [
      {
        "type": "file",
        "id": "101",
        "attributes": {
          "uri": "file:///path/to/src/utils/logging.py",
          "sha256": "abc123...",
          "mime_type": "text/x-python",
          "created_at": "2025-01-10T14:30:00Z",
          "updated_at": "2025-01-10T14:30:00Z"
        },
        "relationships": {
          "authors": {
            "data": [
              {
                "type": "author",
                "id": "202"
              }
            ]
          }
        }
      },
      {
        "type": "author",
        "id": "202",
        "attributes": {
          "name": "Alice Developer",
          "email": "alice@example.com"
        }
      }
    ]
  }
  ```

#### Get Snippet Details

- **Method:** GET
- **Path:** `/api/v1/snippets/{id}`
- **Purpose:** Retrieve full snippet information
- **Headers:** `Authorization: Bearer <token>`
- **Success Response:**

  ```json
  {
    "data": {
      "type": "snippet",
      "id": "789",
      "attributes": {
        "created_at": "2025-01-10T14:30:00Z",
        "updated_at": "2025-01-10T14:30:00Z",
        "original_content": {
          "type": "original",
          "value": "def setup_logger(name: str):\n    logger = logging.getLogger(name)\n    ...",
          "language": "python"
        },
        "summary_content": {
          "type": "summary", 
          "value": "Logger setup utility function",
          "language": "markdown"
        }
      },
      "relationships": {
        "files": {
          "data": [
            {
              "type": "file",
              "id": "101"
            }
          ]
        }
      }
    },
    "included": [
      {
        "type": "file",
        "id": "101",
        "attributes": {
          "uri": "file:///path/to/src/utils/logging.py",
          "sha256": "abc123...",
          "mime_type": "text/x-python",
          "created_at": "2025-01-10T14:30:00Z",
          "updated_at": "2025-01-10T14:30:00Z"
        },
        "relationships": {
          "authors": {
            "data": [
              {
                "type": "author",
                "id": "202"
              }
            ]
          }
        }
      },
      {
        "type": "author",
        "id": "202",
        "attributes": {
          "name": "Alice Developer",
          "email": "alice@example.com"
        }
      }
    ]
  }
  ```

## Implementation Guidelines

### Authentication & Security

**Requirements from User Input:**

- Token-based authentication using Bearer tokens
- Tokens stored in CSV-separated environment variable
- Auto-generation if no tokens provided
- Generated token saved to data directory

**Implementation Approach:**

- Create middleware that intercepts all `/api/*` requests
- Parse `KODIT_API_TOKENS` env var (comma-separated)
- Generate secure token if none exist
- Save to `{data_dir}/api_token.txt` for user reference
- Use constant-time comparison for token validation

**Code References:**

- Extend middleware pattern from `src/kodit/app.py`
- Use config pattern from `src/kodit/config.py`

### Asynchronous Operations

**Requirements from User Input:**

- Index creation must be asynchronous
- Return immediately without status tracking

**Implementation Approach:**

- Use FastAPI's `BackgroundTasks` for indexing
- Use existing `CodeIndexingApplicationService` for all indexing operations
- No status tracking required initially

**Code References:**

- Use async patterns from existing repository implementations
- Follow service patterns from `application/factories/`

### API Feature Parity

**Requirements from User Input:**

- Match all MCP search tool capabilities
- Support same filters and parameters

**Implementation Approach:**

- Reuse domain value objects (`MultiSearchRequest`, `SnippetSearchFilters`)
- Expose all filter options through API schemas
- Maintain same search behavior and scoring

**Code References:**

- Copy search logic from `src/kodit/mcp.py`
- Use same domain services and repositories

## Validation & Testing Strategy

### Functional Validation

**Based on Requirements:**

- [ ] Token authentication blocks unauthorized requests
- [ ] Auto-generated token is saved and printed
- [ ] Index creation returns immediately and starts background indexing
- [ ] Search API matches MCP tool results exactly

**Test Commands:**

```bash
# Test authentication
curl -X GET http://localhost:8000/api/v1/indexes \
  -H "Authorization: Bearer invalid-token" \
  # Should return 401

# Test index creation
curl -X POST http://localhost:8000/api/v1/indexes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"data": {"type": "index", "attributes": {"source_uri": "."}}}'
  # Should return 202 with JSON:API format

# Test search
curl -X POST http://localhost:8000/api/v1/search \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"data": {"type": "search", "attributes": {"keywords": ["def"], "top_k": 5}}}'
  # Should return search results in JSON:API format
```

### Technical Validation

**Based on Architecture:**

- [ ] Clean separation between API and domain layers
- [ ] Repository pattern properly utilized
- [ ] Async operations don't block main thread
- [ ] Error responses follow consistent format

**Test Commands:**

```bash
# Run API-specific tests
uv run pytest tests/kodit/infrastructure/api/ -v

# Check type safety
uv run mypy src/kodit/infrastructure/api/

# Verify OpenAPI schema
curl http://localhost:8000/openapi.json | jq .
```

### User Acceptance Criteria

**Derived from Requirements:**

- [ ] API tokens work as expected with CSV env var
- [ ] Index creation handles large repos without timeout
- [ ] Search results match CLI/MCP output exactly
- [ ] All endpoints documented in OpenAPI/Swagger

## Testing Strategy (TDD Approach)

### Test-Driven Development Plan

Following TDD principles, tests should be written before implementation:

1. **E2E API Tests** - High-level integration tests with real server
2. **Unit Tests** - Individual component tests
3. **Integration Tests** - Service layer tests
4. **Contract Tests** - API schema validation tests

### Test Structure

```
tests/
├── kodit/
│   ├── infrastructure/
│   │   ├── api/
│   │   │   ├── conftest.py           # API test fixtures
│   │   │   ├── e2e/
│   │   │   │   └── test_api_e2e.py   # End-to-end API tests
│   │   │   ├── unit/
│   │   │   │   ├── test_auth_middleware.py    # Auth middleware tests
│   │   │   │   ├── test_json_api_schemas.py   # JSON:API serialization tests
│   │   │   │   └── test_dependencies.py      # FastAPI dependencies tests
│   │   │   └── integration/
│   │   │       ├── test_index_endpoints.py   # Index management tests
│   │   │       └── test_search_endpoints.py  # Search API tests
```

### TDD Test Categories

**High-Level E2E Tests:**
- [ ] Complete workflow: create index → search snippets
- [ ] Authentication flow with token validation
- [ ] JSON:API format compliance
- [ ] Error handling for invalid requests
- [ ] Concurrent request handling

**Unit Tests:**
- [ ] Token authentication middleware
- [ ] JSON:API request/response serialization
- [ ] FastAPI dependencies injection
- [ ] Error response formatting

**Integration Tests:**
- [ ] Index CRUD operations with real database
- [ ] Search functionality with indexed data
- [ ] Filter application and validation
- [ ] Background indexing task execution

### Test Data Strategy

- Use SQLite in-memory database for all tests
- Create sample repositories with realistic code files
- Use minimal test data that covers edge cases
- Focus on light assertions to avoid brittle tests

### Test Fixtures

```python
@pytest.fixture
def auth_headers() -> dict[str, str]:
    return {"Authorization": "Bearer test-token-123"}

@pytest.fixture
def sample_repo_path(tmp_path: Path) -> Path:
    # Creates temporary repo with Python files
    
@pytest.fixture
async def api_app_context() -> AppContext:
    # Test app context with in-memory database
```

## Implementation Roadmap

### Checkpoint 0: Test Infrastructure (TDD)

**Tasks:**
- [x] Create high-level e2e API test with real server and data
- [x] Set up test fixtures and conftest for API testing
- [ ] Add TDD unit tests for authentication middleware
- [ ] Add TDD unit tests for JSON:API serialization
- [ ] Add TDD integration tests for index management endpoints
- [ ] Add TDD integration tests for search endpoints

**Validation:**
```bash
# Run e2e tests
uv run pytest tests/kodit/infrastructure/api/e2e/ -v

# Run all API tests
uv run pytest tests/kodit/infrastructure/api/ -v

# Run with coverage
uv run pytest tests/kodit/infrastructure/api/ --cov=src/kodit/infrastructure/api --cov-report=term-missing
```

### Checkpoint 1: Authentication Infrastructure

**Tasks:**

- [ ] Create API directory structure
- [ ] Implement token auth middleware
- [ ] Add API token configuration
- [ ] Test token generation and validation

**Validation:**

```bash
# Start server and check token generation
uv run kodit serve
# Should print: "🔐 Generated API token: kodit_..."

# Test protected endpoint
curl -I http://localhost:8000/api/v1/indexes
# Should return 401
```

### Checkpoint 2: Index Management API

**Tasks:**

- [ ] Create index schemas and routers
- [ ] Implement CRUD endpoints
- [ ] Add async indexing using application service
- [ ] Connect to existing repositories

**Validation:**

```bash
# Create an index
export TOKEN=$(cat ~/.local/share/kodit/api_token.txt)
curl -X POST http://localhost:8000/api/v1/indexes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"data": {"type": "index", "attributes": {"source_uri": "."}}}'
```

### Checkpoint 3: Search API Implementation

**Tasks:**

- [ ] Create search schemas matching MCP
- [ ] Implement search endpoint
- [ ] Add snippet detail endpoint
- [ ] Ensure feature parity with MCP tool

**Validation:**

```bash
# Test search matches MCP output
uv run kodit search "def main" --limit 5 --format json > mcp_results.json

curl -X POST http://localhost:8000/api/v1/search \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"data": {"type": "search", "attributes": {"keywords": ["def main"], "top_k": 5}}}' > api_results.json

# Compare results (should be identical)
```

## Configuration Updates

### Environment Variables

```python
# In src/kodit/config.py, add:
api_tokens: list[str] = Field(
    default_factory=list,
    description="Comma-separated list of valid API tokens",
    env="KODIT_API_TOKENS"
)

@field_validator("api_tokens", mode="before")
def parse_api_tokens(cls, v):
    if isinstance(v, str) and v:
        return [t.strip() for t in v.split(",") if t.strip()]
    return v or []
```

### Application Integration

```python
# In src/kodit/app.py, add:
from kodit.infrastructure.api.middleware.auth import TokenAuthMiddleware
from kodit.infrastructure.api.v1.routers import indexes, search

# In create_app():
# Add auth middleware
app.add_middleware(
    TokenAuthMiddleware,
    tokens=set(config.api_tokens),
    data_dir=config.data_dir
)

# Include routers
app.include_router(indexes.router)
app.include_router(search.router)
```

## Quick Implementation Guide

### Getting Started (Copy/Paste Ready)

```bash
# 1. Create API structure
mkdir -p src/kodit/infrastructure/api/{v1/{routers,schemas},middleware,utils}
touch src/kodit/infrastructure/api/__init__.py
touch src/kodit/infrastructure/api/v1/__init__.py
touch src/kodit/infrastructure/api/v1/routers/{__init__.py,indexes.py,search.py}
touch src/kodit/infrastructure/api/v1/schemas/{__init__.py,index.py,search.py}
touch src/kodit/infrastructure/api/v1/dependencies.py
touch src/kodit/infrastructure/api/middleware/{__init__.py,auth.py}
touch src/kodit/infrastructure/api/utils/{__init__.py,tokens.py}

# 2. Update configuration
# Add api_tokens field to AppContext in src/kodit/config.py

# 3. Implement authentication middleware
# Copy the TokenAuthMiddleware code to src/kodit/infrastructure/api/middleware/auth.py

# 4. Create routers
# Copy router implementations to respective files

# 5. Update main app
# Modify src/kodit/app.py to include middleware and routers

# 6. Test implementation
uv run kodit serve
# Note the generated token

# Test endpoints
export TOKEN="<generated-token>"
curl -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/v1/indexes
```

### Common Pitfalls & Solutions

- **Pitfall:** Forgetting to make index operations truly async
  **Solution:** Use BackgroundTasks or asyncio.create_task()
  **Example:** See create_index endpoint implementation

- **Pitfall:** Token comparison vulnerable to timing attacks
  **Solution:** Use secrets.compare_digest() for constant-time comparison
  **Example:**

  ```python
  import secrets
  if not secrets.compare_digest(provided_token, valid_token):
      raise HTTPException(401)
  ```

- **Pitfall:** Status tracking lost on server restart
  **Solution:** Consider persistent storage (Redis/DB) for production
  **Example:** Store status in index table with status column
