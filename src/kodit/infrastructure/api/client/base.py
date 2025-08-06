"""Base HTTP client for Kodit API communication."""

import asyncio
from typing import Any

import httpx

from .exceptions import AuthenticationError, KoditAPIError


class BaseAPIClient:
    """Base API client with authentication and retry logic."""

    def __init__(
        self,
        base_url: str,
        api_key: str | None = None,
        timeout: float = 30.0,
        max_retries: int = 3,
        *,
        verify_ssl: bool = True,
    ) -> None:
        """Initialize the API client.

        Args:
            base_url: Base URL of the Kodit server
            api_key: API key for authentication
            timeout: Request timeout in seconds
            max_retries: Maximum retry attempts
            verify_ssl: Whether to verify SSL certificates

        """
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout
        self.max_retries = max_retries
        self.verify_ssl = verify_ssl
        self._client = self._create_client()

    def _create_client(self) -> httpx.AsyncClient:
        """Create the HTTP client with proper configuration."""
        headers = {}
        if self.api_key:
            headers["X-API-Key"] = self.api_key

        return httpx.AsyncClient(
            base_url=self.base_url,
            headers=headers,
            timeout=httpx.Timeout(self.timeout),
            verify=self.verify_ssl,
            follow_redirects=True,
        )

    async def _request(
        self,
        method: str,
        path: str,
        **kwargs: Any,
    ) -> httpx.Response:
        """Make HTTP request with retry logic.

        Args:
            method: HTTP method (GET, POST, etc.)
            path: API endpoint path
            **kwargs: Additional arguments passed to httpx

        Returns:
            HTTP response object

        Raises:
            AuthenticationError: If authentication fails
            KoditAPIError: For other API errors

        """
        url = f"{self.base_url}{path}"

        for attempt in range(self.max_retries):
            try:
                response = await self._client.request(method, url, **kwargs)
                response.raise_for_status()
            except httpx.HTTPStatusError as e:
                if e.response.status_code == 401:
                    raise AuthenticationError("Invalid API key") from e
                if e.response.status_code >= 500 and attempt < self.max_retries - 1:
                    await asyncio.sleep(2**attempt)  # Exponential backoff
                    continue
                raise KoditAPIError(f"API request failed: {e}") from e
            except httpx.RequestError as e:
                if attempt < self.max_retries - 1:
                    await asyncio.sleep(2**attempt)
                    continue
                raise KoditAPIError(f"Connection error: {e}") from e
            else:
                return response

        raise KoditAPIError(f"Max retries ({self.max_retries}) exceeded")

    async def close(self) -> None:
        """Close the HTTP client."""
        await self._client.aclose()
