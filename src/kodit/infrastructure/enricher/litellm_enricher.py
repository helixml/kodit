"""LiteLLM enricher implementation."""

from collections.abc import AsyncGenerator

import structlog

from kodit.config import Endpoint
from kodit.domain.enrichments.enricher import Enricher
from kodit.domain.enrichments.request import EnrichmentRequest
from kodit.domain.enrichments.response import EnrichmentResponse
from kodit.infrastructure.enricher.utils import clean_thinking_tags
from kodit.infrastructure.providers.async_batch_processor import (
    process_items_concurrently,
)
from kodit.infrastructure.providers.litellm_provider import LiteLLMProvider


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
        self.provider: LiteLLMProvider = LiteLLMProvider(endpoint)
        self.endpoint = endpoint

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

        async def process_request(
            request: EnrichmentRequest,
        ) -> EnrichmentResponse:
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
                response.get("choices", [{}])[0].get("message", {}).get("content", "")
            )
            cleaned_content = clean_thinking_tags(content or "")
            return EnrichmentResponse(
                id=request.id,
                text=cleaned_content,
            )

        async for result in process_items_concurrently(
            requests, process_request, self.endpoint.num_parallel_tasks
        ):
            yield result

    async def close(self) -> None:
        """Close the enricher and cleanup HTTPX client if using Unix sockets."""
        await self.provider.close()
