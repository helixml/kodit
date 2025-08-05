"""OpenAI enrichment provider implementation using httpx."""

import asyncio
from collections.abc import AsyncGenerator
from typing import Any

import httpx
import structlog

from kodit.domain.services.enrichment_service import EnrichmentProvider
from kodit.domain.value_objects import EnrichmentRequest, EnrichmentResponse

ENRICHMENT_SYSTEM_PROMPT = """
You are a professional software developer. You will be given a snippet of code.
Please provide a concise explanation of the code.
"""

# Default tuned to approximately fit within OpenAI's rate limit of 500 / RPM
OPENAI_NUM_PARALLEL_TASKS = 40


class OpenAIEnrichmentProvider(EnrichmentProvider):
    """OpenAI enrichment provider implementation using httpx."""

    def __init__(  # noqa: PLR0913
        self,
        api_key: str | None = None,
        base_url: str = "https://api.openai.com",
        model_name: str = "gpt-4o-mini",
        num_parallel_tasks: int = OPENAI_NUM_PARALLEL_TASKS,
        socket_path: str | None = None,
        timeout: float = 30.0,
    ) -> None:
        """Initialize the OpenAI enrichment provider.

        Args:
            api_key: The OpenAI API key.
            base_url: The base URL for the OpenAI API.
            model_name: The model name to use for enrichment.
            num_parallel_tasks: Maximum number of concurrent requests.
            socket_path: Optional Unix socket path for local communication.
            timeout: Request timeout in seconds.

        """
        self.log = structlog.get_logger(__name__)
        self.model_name = model_name
        self.num_parallel_tasks = num_parallel_tasks
        self.api_key = api_key
        self.base_url = base_url
        self.socket_path = socket_path
        self.timeout = timeout

        # Create httpx client with optional Unix socket support
        if socket_path:
            transport = httpx.AsyncHTTPTransport(uds=socket_path)
            self.http_client = httpx.AsyncClient(
                transport=transport,
                base_url="http://localhost",  # Base URL for Unix socket
                timeout=timeout,
            )
        else:
            self.http_client = httpx.AsyncClient(
                base_url=base_url,
                timeout=timeout,
            )

    async def _call_chat_completion(
        self, messages: list[dict[str, str]]
    ) -> dict[str, Any]:
        """Call the chat completion API using httpx.

        Args:
            messages: The messages to send to the API.

        Returns:
            The API response as a dictionary.

        """
        headers = {
            "Content-Type": "application/json",
        }
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        data = {
            "model": self.model_name,
            "messages": messages,
        }

        response = await self.http_client.post(
            "/v1/chat/completions",
            json=data,
            headers=headers,
        )
        response.raise_for_status()
        return response.json()

    async def enrich(
        self, requests: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of requests using OpenAI API.

        Args:
            requests: List of enrichment requests.

        Yields:
            Enrichment responses as they are processed.

        """
        if not requests:
            self.log.warning("No requests for enrichment")
            return

        # Process batches in parallel with a semaphore to limit concurrent requests
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
                    return EnrichmentResponse(
                        snippet_id=request.snippet_id,
                        text=content or "",
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
        """Close the HTTP client."""
        if hasattr(self, "http_client"):
            await self.http_client.aclose()
