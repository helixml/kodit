"""FastAPI application for kodit API."""

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from asgi_correlation_id import CorrelationIdMiddleware
from fastapi import FastAPI

from kodit.application.services.sync_scheduler import SyncSchedulerService
from kodit.config import AppContext
from kodit.infrastructure.indexing.auto_indexing_service import AutoIndexingService
from kodit.mcp import mcp
from kodit.middleware import ASGICancelledErrorMiddleware, logging_middleware

# Global services
_auto_indexing_service: AutoIndexingService | None = None
_sync_scheduler_service: SyncSchedulerService | None = None


@asynccontextmanager
async def app_lifespan(_: FastAPI) -> AsyncIterator[None]:
    """Manage application lifespan for auto-indexing and sync."""
    global _auto_indexing_service, _sync_scheduler_service  # noqa: PLW0603

    app_context = AppContext()
    db = await app_context.get_db()

    # Start auto-indexing service
    _auto_indexing_service = AutoIndexingService(
        app_context=app_context,
        session_factory=db.session_factory,
    )
    await _auto_indexing_service.start_background_indexing()

    # Start sync scheduler service
    if app_context.periodic_sync.enabled:
        _sync_scheduler_service = SyncSchedulerService(
            app_context=app_context,
            session_factory=db.session_factory,
        )
        _sync_scheduler_service.start_periodic_sync(
            interval_seconds=app_context.periodic_sync.interval_seconds
        )

    yield

    # Stop services
    if _sync_scheduler_service:
        await _sync_scheduler_service.stop_periodic_sync()
    if _auto_indexing_service:
        await _auto_indexing_service.stop()


# See https://gofastmcp.com/integrations/fastapi#mounting-an-mcp-server
mcp_sse_app = mcp.http_app(transport="sse", path="/")
mcp_http_app = mcp.http_app(transport="http", path="/")


@asynccontextmanager
async def combined_lifespan(app: FastAPI) -> AsyncIterator[None]:
    """Combine app and MCP lifespans."""
    async with (
        app_lifespan(app),
        mcp_sse_app.router.lifespan_context(app),
        mcp_http_app.router.lifespan_context(app),
    ):
        yield


app = FastAPI(title="kodit API", lifespan=combined_lifespan)

# Add middleware
app.middleware("http")(logging_middleware)
app.add_middleware(CorrelationIdMiddleware)


@app.get("/")
async def root() -> dict[str, str]:
    """Return a welcome message for the kodit API."""
    return {"message": "Hello, World!"}


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    """Return a health check for the kodit API."""
    return {"status": "ok"}


# Add mcp routes last, otherwise previous routes aren't added
# Mount both apps at root - they have different internal paths
app.mount("/sse", mcp_sse_app)
app.mount("/mcp", mcp_http_app)

# Wrap the entire app with ASGI middleware after all routes are added to suppress
# CancelledError at the ASGI level
app = ASGICancelledErrorMiddleware(app)  # type: ignore[assignment]
