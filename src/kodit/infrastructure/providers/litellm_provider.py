"""LiteLLM provider implementation."""

import asyncio
import functools
from collections.abc import Callable, Coroutine
from typing import Any

import httpx
import litellm
import structlog
from litellm import acompletion, aembedding

from kodit.config import Endpoint

ProviderMaxRetriesError = Exception("LiteLLM API error: Max retries exceeded")

RETRYABLE_ERRORS = (
    litellm.exceptions.Timeout,
    litellm.exceptions.RateLimitError,
    litellm.exceptions.InternalServerError,
    litellm.exceptions.ServiceUnavailableError,
    litellm.exceptions.APIConnectionError,
    litellm.exceptions.MidStreamFallbackError,
)


def litellm_retry(
    func: Callable[..., Coroutine[Any, Any, Any]],
) -> Callable[..., Coroutine[Any, Any, Any]]:
    """Retry decorator for LiteLLM API calls with exponential backoff.

    Extracts retry configuration from the endpoint attribute of the first
    argument (self) if it's a LiteLLMProvider instance.
    """

    @functools.wraps(func)
    async def wrapper(*args: Any, **kwargs: Any) -> Any:
        # Extract endpoint configuration from self if available
        endpoint = None
        if args and hasattr(args[0], "endpoint"):
            endpoint = args[0].endpoint

        # Use endpoint configuration or fall back to defaults
        max_retries = endpoint.max_retries if endpoint else 5
        initial_delay = endpoint.initial_delay if endpoint else 2.0
        backoff_factor = endpoint.backoff_factor if endpoint else 2.0

        retries = max_retries
        delay = initial_delay
        log: structlog.stdlib.BoundLogger = structlog.get_logger(__name__)

        while True:
            try:
                return await func(*args, **kwargs)
            except (asyncio.CancelledError, KeyboardInterrupt):
                raise
            except Exception as e:
                if isinstance(e, RETRYABLE_ERRORS) and retries > 0:
                    log.warning(
                        "LiteLLM API error: Retrying",
                        error=e,
                        retries=retries,
                        backoff=delay,
                    )
                    try:
                        await asyncio.sleep(delay)
                    except (asyncio.CancelledError, KeyboardInterrupt):
                        # Cancellation during sleep should stop retries immediately
                        log.info("Retry cancelled during backoff")
                        raise
                    retries -= 1
                    delay *= backoff_factor
                    continue

                exception_info = {
                    attr: getattr(e, attr)
                    for attr in dir(e)
                    if not attr.startswith("_")
                }
                log.exception(
                    "LiteLLM API error, check provider logs for details",
                    error=e,
                    exception_info=exception_info,
                    retries=retries,
                    backoff=delay,
                )
                raise

    return wrapper


class LiteLLMProvider:
    """LiteLLM provider that supports 100+ providers."""

    def __init__(self, endpoint: Endpoint) -> None:
        """Initialize the LiteLLM provider."""
        self.endpoint = endpoint
        self._setup_litellm_client()

    def _setup_litellm_client(self) -> None:
        """Set up LiteLLM with custom HTTPX client for Unix socket support."""
        if self.endpoint.socket_path:
            # Create HTTPX client with Unix socket transport
            transport = httpx.AsyncHTTPTransport(uds=self.endpoint.socket_path)
            unix_client = httpx.AsyncClient(
                transport=transport,
                base_url="http://localhost",  # Base URL for Unix socket
                timeout=self.endpoint.timeout,
            )
            # Set as LiteLLM's async client session
            litellm.aclient_session = unix_client

    def _populate_base_kwargs(self) -> dict[str, Any]:
        """Populate base kwargs common to all API calls."""
        kwargs = {
            "model": self.endpoint.model,
            "timeout": self.endpoint.timeout,
        }

        if self.endpoint.api_key:
            kwargs["api_key"] = self.endpoint.api_key

        if self.endpoint.base_url:
            kwargs["api_base"] = self.endpoint.base_url

        kwargs.update(self.endpoint.extra_params or {})

        return kwargs

    @litellm_retry
    async def chat_completion(self, messages: list[dict[str, str]]) -> Any:
        """Call the chat completion API using LiteLLM."""
        kwargs = self._populate_base_kwargs()
        kwargs["messages"] = messages
        response = await acompletion(max_retries=0, **kwargs)
        return response.model_dump()

    @litellm_retry
    async def embedding(self, texts: list[str]) -> Any:
        """Call the embedding API using LiteLLM."""
        kwargs = self._populate_base_kwargs()
        kwargs["input"] = texts
        response = await aembedding(max_retries=0, **kwargs)
        return response.model_dump()

    async def close(self) -> None:
        """Close the provider and cleanup HTTPX client if using Unix sockets."""
        if (
            self.endpoint.socket_path
            and hasattr(litellm, "aclient_session")
            and litellm.aclient_session
        ):
            await litellm.aclient_session.aclose()
            litellm.aclient_session = None
