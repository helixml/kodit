"""LiteLLM enrichment provider implementation."""

import asyncio
from collections.abc import AsyncGenerator
from typing import Any

import httpx
import litellm
import structlog
from litellm import acompletion

from kodit.config import Endpoint
from kodit.domain.services.enrichment_service import EnrichmentProvider
from kodit.domain.value_objects import EnrichmentRequest, EnrichmentResponse
from kodit.infrastructure.enrichment.utils import clean_thinking_tags

ENRICHMENT_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""

# Default tuned conservatively for broad provider compatibility
DEFAULT_NUM_PARALLEL_TASKS = 20


class LiteLLMEnrichmentProvider(EnrichmentProvider):
    """LiteLLM enrichment provider that supports 100+ providers."""

    def __init__(
        self,
        endpoint: Endpoint,
    ) -> None:
        """Initialize the LiteLLM enrichment provider.

        Args:
            endpoint: The endpoint configuration containing all settings.

        """
        self.log = structlog.get_logger(__name__)
        self.model_name = endpoint.model or "gpt-4o-mini"
        self.api_key = endpoint.api_key
        self.base_url = endpoint.base_url
        self.socket_path = endpoint.socket_path
        self.num_parallel_tasks = (
            endpoint.num_parallel_tasks or DEFAULT_NUM_PARALLEL_TASKS
        )
        self.timeout = endpoint.timeout or 30.0
        self.extra_params = endpoint.extra_params or {}

        # Configure LiteLLM with custom HTTPX client for Unix socket support if needed
        self._setup_litellm_client()

    def _setup_litellm_client(self) -> None:
        """Set up LiteLLM with custom HTTPX client for Unix socket support."""
        if self.socket_path:
            # Create HTTPX client with Unix socket transport
            transport = httpx.AsyncHTTPTransport(uds=self.socket_path)
            unix_client = httpx.AsyncClient(
                transport=transport,
                base_url="http://localhost",  # Base URL for Unix socket
                timeout=self.timeout,
            )
            # Set as LiteLLM's async client session
            litellm.aclient_session = unix_client

    async def _call_chat_completion(self, messages: list[dict[str, str]]) -> Any:
        """Call the chat completion API using LiteLLM.

        Args:
            messages: The messages to send to the API.

        Returns:
            The API response as a dictionary.

        """
        kwargs = {
            "model": self.model_name,
            "messages": messages,
            "timeout": self.timeout,
        }

        # Add API key if provided
        if self.api_key:
            kwargs["api_key"] = self.api_key

        # Add base_url if provided
        if self.base_url:
            kwargs["api_base"] = self.base_url

        # Add extra parameters
        kwargs.update(self.extra_params)

        try:
            # Use litellm's async completion function
            response = await acompletion(**kwargs)
            return (
                response.model_dump() if hasattr(response, "model_dump") else response
            )
        except Exception as e:
            self.log.exception(
                "LiteLLM completion API error", error=str(e), model=self.model_name
            )
            raise

    async def enrich(
        self, requests: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of requests using LiteLLM.

        Args:
            requests: List of enrichment requests.

        Yields:
            Enrichment responses as they are processed.

        """
        if not requests:
            self.log.warning("No requests for enrichment")
            return

        # Process requests in parallel with a semaphore to limit concurrent requests
        sem = asyncio.Semaphore(self.num_parallel_tasks)

        async def process_request(request: EnrichmentRequest) -> EnrichmentResponse:
            async with sem:
                if not request.text:
                    return EnrichmentResponse(
                        snippet_id=request.snippet_id,
                        text="",
                    )
                try:
                    messages = [
                        {
                            "role": "system",
                            "content": ENRICHMENT_SYSTEM_PROMPT,
                        },
                        {"role": "user", "content": request.text},
                    ]
                    response = await self._call_chat_completion(messages)
                    content = (
                        response.get("choices", [{}])[0]
                        .get("message", {})
                        .get("content", "")
                    )
                    # Remove thinking tags from the response
                    cleaned_content = clean_thinking_tags(content or "")
                    return EnrichmentResponse(
                        snippet_id=request.snippet_id,
                        text=cleaned_content,
                    )
                except Exception as e:
                    self.log.exception("Error enriching request", error=str(e))
                    return EnrichmentResponse(
                        snippet_id=request.snippet_id,
                        text="",
                    )

        # Create tasks for all requests
        tasks = [process_request(request) for request in requests]

        # Process all requests and yield results as they complete
        for task in asyncio.as_completed(tasks):
            yield await task

    async def close(self) -> None:
        """Close the provider and cleanup HTTPX client if using Unix sockets."""
        if (
            self.socket_path
            and hasattr(litellm, "aclient_session")
            and litellm.aclient_session
        ):
            await litellm.aclient_session.aclose()
            litellm.aclient_session = None
