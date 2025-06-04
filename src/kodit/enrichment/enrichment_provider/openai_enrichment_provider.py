"""OpenAI embedding service."""

import asyncio

import structlog
import tiktoken
from openai import AsyncOpenAI
from tqdm import tqdm

from kodit.enrichment.enrichment_provider.enrichment_provider import (
    ENRICHMENT_SYSTEM_PROMPT,
    EnrichmentProvider,
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
        self.encoding = tiktoken.encoding_for_model(model_name)

    async def enrich(self, data: list[str]) -> list[str]:
        """Enrich a list of documents."""
        # Process batches in parallel with a semaphore to limit concurrent requests
        sem = asyncio.Semaphore(OPENAI_NUM_PARALLEL_TASKS)

        # Create a list of tuples with a temporary id for each snippet
        # We need to do this so that we can return the results in the same order as the
        # input data
        input_data = [(i, snippet) for i, snippet in enumerate(data)]

        async def process_data(data: tuple[int, str]) -> tuple[int, str]:
            snippet_id, snippet = data
            if not snippet:
                return snippet_id, ""
            async with sem:
                try:
                    response = await self.openai_client.chat.completions.create(
                        model=self.model_name,
                        messages=[
                            {
                                "role": "system",
                                "content": ENRICHMENT_SYSTEM_PROMPT,
                            },
                            {"role": "user", "content": snippet},
                        ],
                    )
                    return snippet_id, response.choices[0].message.content or ""
                except Exception as e:
                    self.log.exception("Error enriching data", error=str(e))
                    return snippet_id, ""

        # Create tasks for all data
        tasks = [process_data(snippet) for snippet in input_data]

        # Process all data and yield results as they complete
        results: list[tuple[int, str]] = []
        for task in tqdm(
            asyncio.as_completed(tasks),
            total=len(tasks),
            leave=False,
        ):
            result = await task
            results.append(result)

        # Output in the same order as the input data
        return [result for _, result in sorted(results, key=lambda x: x[0])]
