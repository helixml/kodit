"""OpenAI enrichment provider implementation."""

import asyncio
from collections.abc import AsyncGenerator
from typing import Any

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
    """OpenAI enrichment provider implementation."""

    def __init__(self, openai_client: Any, model_name: str = "gpt-4o-mini") -> None:
        """Initialize the OpenAI enrichment provider.

        Args:
            openai_client: The OpenAI client instance.
            model_name: The model name to use for enrichment.

        """
        self.log = structlog.get_logger(__name__)
        self.openai_client = openai_client
        self.model_name = model_name

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
        sem = asyncio.Semaphore(OPENAI_NUM_PARALLEL_TASKS)

        async def process_request(request: EnrichmentRequest) -> EnrichmentResponse:
            async with sem:
                if not request.text:
                    return EnrichmentResponse(
                        snippet_id=request.snippet_id,
                        text="",
                    )
                try:
                    response = await self.openai_client.chat.completions.create(
                        model=self.model_name,
                        messages=[
                            {
                                "role": "system",
                                "content": ENRICHMENT_SYSTEM_PROMPT,
                            },
                            {"role": "user", "content": request.text},
                        ],
                    )
                    return EnrichmentResponse(
                        snippet_id=request.snippet_id,
                        text=response.choices[0].message.content or "",
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
