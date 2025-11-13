"""FastAPI application for kodit API."""

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

import structlog
from asgi_correlation_id import CorrelationIdMiddleware
from fastapi import FastAPI, Response
from fastapi.responses import RedirectResponse

from kodit._version import version
from kodit.application.factories.reporting_factory import create_server_operation
from kodit.application.factories.server_factory import ServerFactory
from kodit.application.services.indexing_worker_service import IndexingWorkerService
from kodit.application.services.sync_scheduler import SyncSchedulerService
from kodit.config import AppContext
from kodit.domain.enrichments.request import EnrichmentRequest
from kodit.domain.value_objects import EmbeddingRequest
from kodit.infrastructure.api.v1.routers.commits import router as commits_router
from kodit.infrastructure.api.v1.routers.queue import router as queue_router
from kodit.infrastructure.api.v1.routers.repositories import (
    router as repositories_router,
)
from kodit.infrastructure.api.v1.routers.search import router as search_router
from kodit.infrastructure.api.v1.schemas.context import AppLifespanState
from kodit.infrastructure.sqlalchemy.task_status_repository import (
    create_task_status_repository,
)
from kodit.mcp import mcp
from kodit.middleware import (
    ASGICancelledErrorMiddleware,
    logging_middleware,
)

# Global services
_sync_scheduler_service: SyncSchedulerService | None = None
_server_factory: ServerFactory | None = None


@asynccontextmanager
async def app_lifespan(_: FastAPI) -> AsyncIterator[AppLifespanState]:
    """Manage application lifespan for auto-indexing and sync."""
    global _sync_scheduler_service  # noqa: PLW0603
    global _server_factory  # noqa: PLW0603

    # App context has already been configured by the CLI.
    app_context = AppContext()
    db = await app_context.get_db()
    log = structlog.get_logger(__name__)
    operation = create_server_operation(
        create_task_status_repository(db.session_factory)
    )

    _server_factory = ServerFactory(app_context, db.session_factory)

    # Quickly check if the providers are accessible and raise an error if not
    log.info("Checking providers are accessible")

    # Check embedding provider directly
    try:
        embedding_service = _server_factory.code_search_service()
        await anext(
            embedding_service.embedding_provider.embed(
                [EmbeddingRequest(snippet_id="1", text="test")]
            )
        )
    except StopAsyncIteration:
        pass
    except Exception as e:
        raise ValueError("Embedding service is not accessible") from e

    # Check enrichment provider directly
    try:
        enricher = _server_factory.enricher()
        await anext(
            enricher.enrich(
                [
                    EnrichmentRequest(
                        id="1",
                        text="test",
                        system_prompt="Reply with OK",
                    )
                ]
            )
        )
    except Exception as e:
        raise ValueError("Enrichment service is not accessible") from e

    # Start the queue worker service
    _indexing_worker_service = IndexingWorkerService(
        app_context=app_context,
        session_factory=db.session_factory,
        server_factory=_server_factory,
    )
    await _indexing_worker_service.start(operation)

    # Start sync scheduler service
    if app_context.periodic_sync.enabled:
        _sync_scheduler_service = _server_factory.sync_scheduler_service()
        _sync_scheduler_service.start_periodic_sync(
            interval_seconds=app_context.periodic_sync.interval_seconds
        )

    yield AppLifespanState(app_context=app_context, server_factory=_server_factory)

    # Stop services
    if _sync_scheduler_service:
        await _sync_scheduler_service.stop_periodic_sync()
    if _indexing_worker_service:
        await _indexing_worker_service.stop()


# See https://gofastmcp.com/integrations/fastapi#mounting-an-mcp-server
mcp_sse_app = mcp.http_app(transport="sse", path="/")
mcp_http_app = mcp.http_app(transport="http", path="/")


@asynccontextmanager
async def combined_lifespan(app: FastAPI) -> AsyncIterator[AppLifespanState]:
    """Combine app and MCP lifespans, yielding state from app_lifespan."""
    async with (
        app_lifespan(app) as app_state,
        mcp_sse_app.router.lifespan_context(app),
        mcp_http_app.router.lifespan_context(app),
    ):
        yield app_state


app = FastAPI(
    title="kodit API",
    lifespan=combined_lifespan,
    responses={
        500: {"description": "Internal server error"},
    },
    description="""
This is the REST API for the Kodit server. Please refer to the
[Kodit documentation](https://docs.helix.ml/kodit/) for more information.
    """,
    version=version,
)

# Add middleware. Remember, last runs first. Order is important.
app.middleware("http")(logging_middleware)  # Then always log
app.add_middleware(CorrelationIdMiddleware)  # Add correlation id first.


@app.get("/", include_in_schema=False)
async def root() -> RedirectResponse:
    """Redirect to the API documentation."""
    return RedirectResponse(url="/docs")


@app.get("/healthz")
async def healthz() -> Response:
    """Return a health check for the kodit API."""
    return Response(status_code=200)


# Include API routers
app.include_router(queue_router)
app.include_router(search_router)
app.include_router(commits_router)
app.include_router(repositories_router)

# Add mcp routes last, otherwise previous routes aren't added
# Mount both apps at root - they have different internal paths
app.mount("/sse", mcp_sse_app)
app.mount("/mcp", mcp_http_app)

# Wrap the entire app with ASGI middleware after all routes are added to suppress
# CancelledError at the ASGI level
ASGICancelledErrorMiddleware(app)
