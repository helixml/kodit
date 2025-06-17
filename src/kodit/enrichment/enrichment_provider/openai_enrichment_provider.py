"""OpenAI embedding service."""

import asyncio
from collections.abc import AsyncGenerator

import structlog
import tiktoken
from openai import AsyncOpenAI

from kodit.enrichment.enrichment_provider.enrichment_provider import (
    ENRICHMENT_SYSTEM_PROMPT,
    EnrichmentProvider,
    EnrichmentRequest,
    EnrichmentResponse,
)

OPENAI_NUM_PARALLEL_TASKS = 10


class OpenAIEnrichmentProvider(EnrichmentProvider):
    """OpenAI enrichment provider."""

    def __init__(
        self,
        openai_client: AsyncOpenAI,
        model_name: str = "gpt-4o-mini",
    ) -> None:
        """Initialize the OpenAI enrichment provider."""
        self.log = structlog.get_logger(__name__)
        self.openai_client = openai_client
        self.model_name = model_name
        self.encoding = tiktoken.encoding_for_model("gpt-4o-mini")  # Approximation

    async def enrich(
        self, data: list[EnrichmentRequest]
    ) -> AsyncGenerator[EnrichmentResponse, None]:
        """Enrich a list of documents."""
        if not data or len(data) == 0:
            self.log.warning("Data is empty, skipping enrichment")
            return

        # Process batches in parallel with a semaphore to limit concurrent requests
        sem = asyncio.Semaphore(OPENAI_NUM_PARALLEL_TASKS)

        async def process_data(data: EnrichmentRequest) -> EnrichmentResponse:
            async with sem:
                if not data.text:
                    return EnrichmentResponse(
                        snippet_id=data.snippet_id,
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
                            {"role": "user", "content": data.text},
                        ],
                    )
                    return EnrichmentResponse(
                        snippet_id=data.snippet_id,
                        text=response.choices[0].message.content or "",
                    )
                except Exception as e:
                    self.log.exception("Error enriching data", error=str(e))
                    return EnrichmentResponse(
                        snippet_id=data.snippet_id,
                        text="",
                    )

        # Create tasks for all data
        tasks = [process_data(snippet) for snippet in data]

        # Process all data and yield results as they complete
        for task in asyncio.as_completed(tasks):
            yield await task
