"""FastAPI application for kodit API."""

from asgi_correlation_id import CorrelationIdMiddleware
from fastapi import FastAPI

from kodit.mcp import mcp
from kodit.middleware import ASGICancelledErrorMiddleware, logging_middleware

# See https://gofastmcp.com/deployment/asgi#fastapi-integration
mcp_app = mcp.sse_app()
app = FastAPI(title="kodit API", lifespan=mcp_app.router.lifespan_context)

# Add middleware
app.middleware("http")(logging_middleware)
app.add_middleware(CorrelationIdMiddleware)


@app.get("/")
async def root() -> dict[str, str]:
    """Return a welcome message for the kodit API."""
    return {"message": "Hello, World!"}


# Add mcp routes last, otherwise previous routes aren't added
app.mount("", mcp_app)

# Wrap the entire app with ASGI middleware after all routes are added to suppress
# CancelledError at the ASGI level
app = ASGICancelledErrorMiddleware(app)
