"""LiteLLM enricher implementation."""

import asyncio
from collections.abc import AsyncGenerator

import structlog

from kodit.config import Endpoint
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.request import EnrichmentRequest
from kodit.domain.enrichments.response import EnrichmentResponse
from kodit.infrastructure.enricher.utils import clean_thinking_tags
from kodit.infrastructure.providers.litellm_provider import LiteLLMProvider

DEFAULT_NUM_PARALLEL_TASKS = 20


class LiteLLMEnricher(Enricher):
    """LiteLLM enricher that supports 100+ providers."""

    def __init__(
        self,
        endpoint: Endpoint,
    ) -> None:
        """Initialize the LiteLLM enricher.

        Args:
            endpoint: The endpoint configuration containing all settings.

        """
        self.log = structlog.get_logger(__name__)
        self.num_parallel_tasks = (
            endpoint.num_parallel_tasks or DEFAULT_NUM_PARALLEL_TASKS
        )
        self.provider: LiteLLMProvider = LiteLLMProvider(endpoint)

    async def enrich(
        self, requests: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of requests using LiteLLM.

        Args:
            requests: List of generic enrichment requests.

        Yields:
            Generic enrichment responses as they are processed.

        """
        if not requests:
            self.log.warning("No requests for enrichment")
            return

        sem = asyncio.Semaphore(self.num_parallel_tasks)

        async def process_request(
            request: EnrichmentRequest,
        ) -> EnrichmentResponse:
            async with sem:
                if not request.text:
                    return EnrichmentResponse(
                        id=request.id,
                        text="",
                    )
                messages = [
                    {
                        "role": "system",
                        "content": request.system_prompt,
                    },
                    {"role": "user", "content": request.text},
                ]
                response = await self.provider.chat_completion(messages)
                content = (
                    response.get("choices", [{}])[0]
                    .get("message", {})
                    .get("content", "")
                )
                cleaned_content = clean_thinking_tags(content or "")
                return EnrichmentResponse(
                    id=request.id,
                    text=cleaned_content,
                )

        tasks: list[asyncio.Task[EnrichmentResponse]] = [
            asyncio.create_task(process_request(request)) for request in requests
        ]

        try:
            for task in asyncio.as_completed(tasks):
                yield await task
        finally:
            # Cancel any remaining tasks when generator exits
            # (due to exception, Ctrl+C, or early consumer termination)
            for task in tasks:
                if not task.done():
                    task.cancel()

            # Wait for all tasks to finish cancelling
            await asyncio.gather(*tasks, return_exceptions=True)

    async def close(self) -> None:
        """Close the enricher and cleanup HTTPX client if using Unix sockets."""
        await self.provider.close()
