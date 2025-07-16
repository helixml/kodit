"""API key-based authentication middleware for the REST API."""

from fastapi import Depends, HTTPException, Request, Security
from fastapi.security import APIKeyHeader

api_key_header_value = APIKeyHeader(
    name="x-api-key",
    auto_error=False,
    description="API key for authentication (only if set in environmental variables)",
    scheme_name="Header (X-API-KEY)",
)


def valid_keys(request: Request) -> list[str]:
    """Get the valid keys from the app context."""
    if not hasattr(request.state, "app_context"):
        raise HTTPException(status_code=500, detail="App context not found")
    app_context = request.state.app_context
    return app_context.api_keys


def api_key_auth(
    api_key: str = Security(api_key_header_value),
    valid_keys: list[str] = Depends(valid_keys),
) -> None:
    """Validate the API key."""
    if len(valid_keys) == 0:
        return
    # Check if the API key is valid
    if api_key not in valid_keys:
        raise HTTPException(
            status_code=401,
            detail="Invalid API key",
        )
