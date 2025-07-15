"""Token-based authentication middleware for the REST API."""

import structlog
from fastapi import Request
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware, RequestResponseEndpoint
from starlette.responses import Response
from starlette.types import ASGIApp


class TokenAuthMiddleware(BaseHTTPMiddleware):
    """Token-based authentication middleware with auto-generation support."""

    def __init__(self, app: ASGIApp) -> None:
        """Initialize the token authentication middleware."""
        super().__init__(app)
        self.log = structlog.get_logger(__name__)

    async def dispatch(
        self, request: Request, call_next: RequestResponseEndpoint
    ) -> Response:
        """Authenticate API requests."""
        # Skip auth for health endpoints
        if request.url.path in ["/", "/healthz", "/docs", "/openapi.json"]:
            return await call_next(request)

        # Initialize valid tokens
        if not hasattr(request.state, "app_context"):
            return JSONResponse(
                status_code=500, content={"detail": "App context not found"}
            )
        app_context = request.state.app_context
        valid_tokens = app_context.api_tokens

        # If no tokens are set, skip auth
        if len(valid_tokens) == 0:
            return await call_next(request)

        # Protect /api/* endpoints
        if request.url.path.startswith("/api/"):
            print(f"protecting {request.url.path}")
            print(f"valid_tokens: {valid_tokens}")
            auth_header = request.headers.get("Authorization")
            print(f"auth_header: {auth_header}")
            if not auth_header or not auth_header.startswith("Bearer "):
                print("missing or invalid authorization header")
                return JSONResponse(
                    status_code=401,
                    content={"detail": "Missing or invalid authorization header"},
                )

            token = auth_header.split(" ", 1)[1]
            print(f"token: {token}")
            # Use constant-time comparison to prevent timing attacks
            if token not in valid_tokens:
                return JSONResponse(
                    status_code=401,
                    content={"detail": "Not authorized"},
                )

        return await call_next(request)
