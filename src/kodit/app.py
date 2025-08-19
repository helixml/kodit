"""FastAPI application for kodit API."""

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from asgi_correlation_id import CorrelationIdMiddleware
from fastapi import FastAPI, Response
from fastapi.responses import RedirectResponse

from kodit._version import version
from kodit.application.services.auto_indexing_service import AutoIndexingService
from kodit.application.services.indexing_worker_service import IndexingWorkerService
from kodit.application.services.sync_scheduler import SyncSchedulerService
from kodit.config import AppContext
from kodit.infrastructure.api.v1.routers import (
    indexes_router,
    queue_router,
    search_router,
)
from kodit.infrastructure.api.v1.schemas.context import AppLifespanState
from kodit.mcp import mcp
from kodit.middleware import ASGICancelledErrorMiddleware, logging_middleware

# Global services
_auto_indexing_service: AutoIndexingService | None = None
_sync_scheduler_service: SyncSchedulerService | None = None


@asynccontextmanager
async def app_lifespan(_: FastAPI) -> AsyncIterator[AppLifespanState]:
    """Manage application lifespan for auto-indexing and sync."""
    global _auto_indexing_service, _sync_scheduler_service  # noqa: PLW0603

    # App context has already been configured by the CLI.
    app_context = AppContext()
    db = await app_context.get_db()

    # Start the queue worker service
    _indexing_worker_service = IndexingWorkerService(
        app_context=app_context,
        session_factory=db.session_factory,
    )
    await _indexing_worker_service.start()

    # Start auto-indexing service
    _auto_indexing_service = AutoIndexingService(
        app_context=app_context,
        session_factory=db.session_factory,
    )
    await _auto_indexing_service.start_background_indexing()

    # Start sync scheduler service
    if app_context.periodic_sync.enabled:
        _sync_scheduler_service = SyncSchedulerService(
            session_factory=db.session_factory,
        )
        _sync_scheduler_service.start_periodic_sync(
            interval_seconds=app_context.periodic_sync.interval_seconds
        )

    yield AppLifespanState(app_context=app_context)

    # Stop services
    if _sync_scheduler_service:
        await _sync_scheduler_service.stop_periodic_sync()
    if _auto_indexing_service:
        await _auto_indexing_service.stop()
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
app.include_router(indexes_router)
app.include_router(queue_router)
app.include_router(search_router)


# Add mcp routes last, otherwise previous routes aren't added
# Mount both apps at root - they have different internal paths
app.mount("/sse", mcp_sse_app)
app.mount("/mcp", mcp_http_app)

# Wrap the entire app with ASGI middleware after all routes are added to suppress
# CancelledError at the ASGI level
ASGICancelledErrorMiddleware(app)
