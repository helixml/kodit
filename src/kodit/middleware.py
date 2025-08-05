"""Middleware for the FastAPI application."""

import contextlib
import time
from asyncio import CancelledError
from collections.abc import Callable

import structlog
from asgi_correlation_id.context import correlation_id
from fastapi import Request, Response
from starlette.types import ASGIApp, Receive, Scope, Send

access_logger = structlog.stdlib.get_logger("api.access")


async def logging_middleware(request: Request, call_next: Callable) -> Response:
    """Log HTTP requests and responses.

    This middleware logs HTTP requests and responses, including timing information
    and request IDs.
    """
    structlog.contextvars.clear_contextvars()
    # These context vars will be added to all log entries emitted during the request
    request_id = correlation_id.get()
    structlog.contextvars.bind_contextvars(request_id=request_id)

    start_time = time.perf_counter_ns()
    # If the call_next raises an error, we still want to return our own 500 response,
    # so we can add headers to it (process time, request ID...)
    response = Response(status_code=500)
    try:
        response = await call_next(request)
    except Exception:
        structlog.stdlib.get_logger("api.error").exception("Uncaught exception")
        raise
    finally:
        process_time = time.perf_counter_ns() - start_time
        status_code = response.status_code
        client_host = request.client.host if request.client else None
        client_port = request.client.port if request.client else None
        http_method = request.method
        http_version = request.scope["http_version"]
        # Recreate the Uvicorn access log format, but add all parameters as
        # structured information
        access_logger.info(
            "Request processed",
            http={
                "url": str(request.url),
                "status_code": status_code,
                "method": http_method,
                "request_id": request_id,
                "version": http_version,
                "client_host": client_host,
                "client_port": client_port,
            },
            headers=dict(request.headers),
            network={"client": {"ip": client_host, "port": client_port}},
            duration=process_time,
        )
        response.headers["X-Process-Time"] = str(process_time / 10**9)

    return response


class ASGICancelledErrorMiddleware:
    """ASGI middleware to handle CancelledError at the ASGI level."""

    def __init__(self, app: ASGIApp) -> None:
        """Initialize the middleware."""
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        """Handle the ASGI request and catch CancelledError."""
        with contextlib.suppress(CancelledError):
            await self.app(scope, receive, send)
