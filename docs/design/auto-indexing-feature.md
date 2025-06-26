# Auto-Indexing Feature Design Document

## Overview

This document outlines the design for a new auto-indexing feature that allows users to specify which sources should be indexed via configuration, rather than requiring manual CLI commands for each repository.

## Problem Statement

Currently, users must manually run `kodit index <source>` for each repository they want to index. This creates friction for users who want to maintain a consistent set of indexed repositories across different environments or want to automatically index repositories when starting the server.

## Goals

1. **Configuration-driven indexing**: Allow users to specify repositories to index via environment variables
2. **CLI integration**: Maintain the existing `kodit index` command behavior while adding auto-indexing capabilities
3. **Server mode integration**: Automatically index configured repositories when the server starts
4. **Backward compatibility**: Ensure existing functionality remains unchanged
5. **Error handling**: Gracefully handle indexing failures without breaking the application

## Non-Goals

- Scheduled re-indexing
- Per-source configuration (schedules, retry policies, priorities)
- Health monitoring
- Dependency management
- Configuration files (JSON/YAML)

## Design

### Configuration Schema

#### Structured Configuration Model

Create a structured configuration schema that supports the core functionality and is future-proof for additional options:

```python
# src/kodit/config.py
from typing import Optional
from pydantic import BaseModel, Field

class AutoIndexingSource(BaseModel):
    """Configuration for a single auto-indexing source."""
    
    uri: str = Field(description="URI of the source to index (git URL or local path)")

class AutoIndexingConfig(BaseModel):
    """Configuration for auto-indexing."""
    
    sources: list[AutoIndexingSource] = Field(
        default_factory=list,
        description="List of sources to auto-index"
    )
    
    @property
    def enabled(self) -> bool:
        """Whether auto-indexing is enabled (has sources configured)."""
        return len(self.sources) > 0

class AppContext(BaseSettings):
    """Global context for the kodit project. Provides a shared state for the app."""
    
    # ... existing fields ...
    
    auto_indexing: AutoIndexingConfig = Field(
        default_factory=AutoIndexingConfig,
        description="Auto-indexing configuration"
    )
```

#### Environment Variable Support

Support configuration via environment variables using the existing Pydantic Settings nested configuration:

```bash
# Configure sources using indexed environment variables with structured objects
KODIT_AUTO_INDEXING_SOURCES_0_URI="https://github.com/pydantic/pydantic"
KODIT_AUTO_INDEXING_SOURCES_1_URI="https://github.com/fastapi/fastapi"
KODIT_AUTO_INDEXING_SOURCES_2_URI="/local/path/to/repo"
```

This structured approach allows for future expansion of source configuration:

```bash
# Future-proof for additional source configuration
KODIT_AUTO_INDEXING_SOURCES_0_URI="https://github.com/pydantic/pydantic"
KODIT_AUTO_INDEXING_SOURCES_0_BRANCH="main"
KODIT_AUTO_INDEXING_SOURCES_0_SCHEDULE="daily"
KODIT_AUTO_INDEXING_SOURCES_1_URI="https://github.com/fastapi/fastapi"
KODIT_AUTO_INDEXING_SOURCES_1_BRANCH="develop"
```

### CLI Integration

#### Enhanced CLI with Auto-Index Flag

```python
@with_app_context
@with_session
async def index(
    session: AsyncSession,
    app_context: AppContext,
    sources: list[str],
    auto_index: bool,
) -> None:
    """List indexes, or index data sources."""
    # ... existing logic for listing indexes ...
    
    if auto_index:
        auto_sources = app_context.get_auto_index_sources()
        if not auto_sources:
            click.echo("No auto-index sources configured.")
            return
        
        click.echo(f"Auto-indexing {len(auto_sources)} configured sources...")
        sources = auto_sources
    
    # ... rest of existing logic for indexing sources ...
```

### Server Mode Integration

#### Simple Background Indexing Service

```python
# src/kodit/infrastructure/indexing/auto_indexing_service.py
import asyncio
from typing import Optional
import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.code_indexing_application_service import CodeIndexingApplicationService
from kodit.domain.services.source_service import SourceService

class AutoIndexingService:
    """Service for automatically indexing configured sources."""
    
    def __init__(
        self,
        app_context: AppContext,
        session_factory: Callable[[], AsyncSession],
    ):
        self.app_context = app_context
        self.session_factory = session_factory
        self.log = structlog.get_logger(__name__)
        self._indexing_task: Optional[asyncio.Task] = None
    
    async def start_background_indexing(self) -> None:
        """Start background indexing of configured sources."""
        if not self.app_context.auto_indexing.enabled:
            self.log.info("Auto-indexing is disabled (no sources configured)")
            return
        
        auto_sources = self.app_context.get_auto_index_sources()
        self.log.info("Starting background indexing", num_sources=len(auto_sources))
        self._indexing_task = asyncio.create_task(self._index_sources(auto_sources))
    
    async def _index_sources(self, sources: list[str]) -> None:
        """Index all configured sources in the background."""
        async with self.session_factory() as session:
            source_service = SourceService(
                clone_dir=self.app_context.get_clone_dir(),
                session_factory=lambda: session,
            )
            
            service = create_code_indexing_application_service(
                app_context=self.app_context,
                session=session,
                source_service=source_service,
            )
            
            for source in sources:
                try:
                    self.log.info("Auto-indexing source", source=source)
                    
                    # Create source
                    s = await source_service.create(source)
                    
                    # Create index
                    index = await service.create_index(s.id)
                    
                    # Run indexing (without progress callback for background mode)
                    await service.run_index(index.id, progress_callback=None)
                    
                    self.log.info("Successfully auto-indexed source", source=source)
                    
                except Exception as e:
                    self.log.error("Failed to auto-index source", source=source, error=str(e))
                    # Continue with other sources even if one fails
    
    async def stop(self) -> None:
        """Stop background indexing."""
        if self._indexing_task:
            self._indexing_task.cancel()
            try:
                await self._indexing_task
            except asyncio.CancelledError:
                pass
```

#### Server Integration

Update the FastAPI application to start the auto-indexing service:

```python
# src/kodit/app.py
from contextlib import asynccontextmanager
from typing import AsyncIterator

from fastapi import FastAPI
from kodit.infrastructure.indexing.auto_indexing_service import AutoIndexingService

auto_indexing_service: Optional[AutoIndexingService] = None

@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    """Application lifespan manager."""
    global auto_indexing_service
    
    # Start auto-indexing service
    app_context = AppContext()
    db = await app_context.get_db()
    auto_indexing_service = AutoIndexingService(
        app_context=app_context,
        session_factory=db.session_factory,
    )
    await auto_indexing_service.start_background_indexing()
    
    yield
    
    # Stop auto-indexing service
    if auto_indexing_service:
        await auto_indexing_service.stop()

app = FastAPI(
    title="kodit API", 
    lifespan=lifespan
)
```

### Error Handling

#### Graceful Degradation

1. **Individual source failures**: If one source fails to index, continue with others
2. **Configuration errors**: Log warnings for invalid sources but don't crash
3. **Network issues**: Log errors but don't retry automatically
4. **Authentication issues**: Clear error messages for authentication problems

#### Logging Strategy

```python
# Different log levels for different scenarios
self.log.info("Starting auto-indexing", num_sources=len(sources))
self.log.info("Successfully indexed source", source=source)
self.log.error("Failed to index source", source=source, error=str(e))
```

### Testing Strategy

#### Unit Tests

1. **Configuration parsing**: Test `get_auto_index_sources()` with various inputs
2. **AutoIndexingService**: Test indexing logic and error handling
3. **CLI integration**: Test `--auto-index` flag behavior

#### Integration Tests

1. **End-to-end indexing**: Test complete auto-indexing flow
2. **Server startup**: Test auto-indexing starts with server
3. **Error scenarios**: Test behavior with invalid sources

#### Test Data

```python
# tests/kodit/infrastructure/indexing/test_auto_indexing_service.py
@pytest.fixture
def mock_sources():
    return [
        AutoIndexingSource(uri="https://github.com/test/repo1"),
        AutoIndexingSource(uri="https://github.com/test/repo2"),
        AutoIndexingSource(uri="/local/test/path")
    ]

@pytest.fixture
def app_context_with_sources(mock_sources):
    return AppContext(auto_indexing=AutoIndexingConfig(sources=mock_sources))
```

## Implementation Plan

### Phase 1: Configuration and CLI (Week 1)

1. Update `AppContext` configuration class
2. Add `--auto-index` flag to CLI
3. Implement configuration parsing logic
4. Add unit tests for configuration

### Phase 2: Background Service (Week 2)

1. Create `AutoIndexingService` class
2. Implement background indexing logic
3. Add error handling and logging
4. Add unit tests for service

### Phase 3: Server Integration (Week 3)

1. Integrate auto-indexing with FastAPI lifespan
2. Update MCP server integration
3. Add integration tests
4. Performance testing

### Phase 4: Documentation and Polish (Week 4)

1. Update documentation
2. Add examples and usage guides
3. Performance optimization
4. Final testing and bug fixes

## Configuration Examples

### Development Environment

```bash
# Environment variables
KODIT_AUTO_INDEXING_SOURCES_0_URI=https://github.com/myorg/core-lib
KODIT_AUTO_INDEXING_SOURCES_1_URI=/home/user/projects/my-app
```

### Production Environment

```bash
# Docker environment
KODIT_AUTO_INDEXING_SOURCES_0_URI=https://github.com/company/backend
KODIT_AUTO_INDEXING_SOURCES_1_URI=https://github.com/company/frontend
KODIT_AUTO_INDEXING_SOURCES_2_URI=https://github.com/company/shared-libs
```

## Migration Strategy

### Backward Compatibility

1. **Existing CLI behavior**: Unchanged - `kodit index` still works as before
2. **No configuration**: If no auto-indexing is configured, no auto-indexing occurs
3. **Gradual adoption**: Users can adopt the feature incrementally

## Conclusion

This simplified design provides a focused, configuration-driven approach to automatic indexing that integrates seamlessly with both CLI and server modes. The implementation maintains backward compatibility while providing a smooth user experience for managing indexed repositories.

The feature addresses the core pain point of manual indexing with a minimal, maintainable implementation that can be extended in the future as needed.
