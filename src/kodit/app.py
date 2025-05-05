"""FastAPI application for kodit API."""

from asgi_correlation_id import CorrelationIdMiddleware
from fastapi import FastAPI

from kodit.mcp import mcp
from kodit.middleware import logging_middleware
from kodit.sse import create_sse_server

app = FastAPI(title="kodit API")

# Get the SSE routes from the Starlette app hosting the MCP server
sse_app = create_sse_server(mcp)
for route in sse_app.routes:
    app.router.routes.append(route)

# Add middleware
app.middleware("http")(logging_middleware)
app.add_middleware(CorrelationIdMiddleware)


@app.get("/")
async def root() -> dict[str, str]:
    """Return a welcome message for the kodit API."""
    return {"message": "Welcome to kodit API"}
