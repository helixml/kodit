"""Token-based authentication middleware for the REST API."""

import secrets
from pathlib import Path

from fastapi import HTTPException, Request
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import Response


class TokenAuthMiddleware(BaseHTTPMiddleware):
    """Token-based authentication middleware with auto-generation support."""

    def __init__(self, app, tokens: set[str], data_dir: Path) -> None:
        """Initialize the token authentication middleware."""
        super().__init__(app)
        self.valid_tokens = tokens
        self.data_dir = data_dir
        self.token_file = self.data_dir / "api_token.txt"

        # Auto-generate token if none provided
        if not self.valid_tokens:
            token = self._get_or_generate_token()
            self.valid_tokens.add(token)

    def _get_or_generate_token(self) -> str:
        """Get existing token or generate a new one."""
        if self.token_file.exists():
            try:
                return self.token_file.read_text().strip()
            except Exception:
                pass  # Fall through to generate new token

        return self._generate_and_save_token()

    def _generate_and_save_token(self) -> str:
        """Generate and save a new API token."""
        token = f"kodit_{secrets.token_urlsafe(32)}"
        self.token_file.parent.mkdir(parents=True, exist_ok=True)
        self.token_file.write_text(token)
        return token

    async def dispatch(self, request: Request, call_next) -> Response:
        """Authenticate API requests."""
        # Skip auth for health endpoints
        if request.url.path in ["/", "/healthz"]:
            return await call_next(request)

        # Only protect /api/* endpoints
        if request.url.path.startswith("/api/"):
            auth_header = request.headers.get("Authorization")
            if not auth_header or not auth_header.startswith("Bearer "):
                raise HTTPException(
                    status_code=401, detail="Missing or invalid authorization header"
                )

            token = auth_header.split(" ", 1)[1]

            # Use constant-time comparison to prevent timing attacks
            valid_token_found = False
            for valid_token in self.valid_tokens:
                if secrets.compare_digest(token, valid_token):
                    valid_token_found = True
                    break

            if not valid_token_found:
                raise HTTPException(status_code=401, detail="Invalid token")

        return await call_next(request)
