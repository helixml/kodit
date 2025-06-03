"""OpenAI embedding service."""

import asyncio

import structlog
import tiktoken
from openai import AsyncOpenAI

from kodit.embedding.embedding_provider.embedding_provider import (
    EmbeddingProvider,
    Vector,
    split_sub_batches,
)

OPENAI_NUM_PARALLEL_TASKS = 10


class OpenAIEmbeddingProvider(EmbeddingProvider):
    """OpenAI embedder."""

    def __init__(
        self,
        openai_client: AsyncOpenAI,
        model_name: str = "text-embedding-3-small",
    ) -> None:
        """Initialize the OpenAI embedder."""
        self.log = structlog.get_logger(__name__)
        self.openai_client = openai_client
        self.model_name = model_name
        self.encoding = tiktoken.encoding_for_model(model_name)

    async def embed(self, data: list[str]) -> list[Vector]:
        """Embed a list of documents."""
        # First split the list into a list of list where each sublist has fewer than
        # max tokens.
        batched_data = split_sub_batches(self.encoding, data)

        # Process batches in parallel with a semaphore to limit concurrent requests
        sem = asyncio.Semaphore(OPENAI_NUM_PARALLEL_TASKS)

        async def process_batch(batch: list[str]) -> list[Vector]:
            async with sem:
                try:
                    response = await self.openai_client.embeddings.create(
                        model=self.model_name,
                        input=batch,
                    )
                    return [
                        [float(x) for x in embedding.embedding]
                        for embedding in response.data
                    ]
                except Exception as e:
                    self.log.exception("Error embedding batch", error=str(e))
                    return []

        # Create tasks for all batches
        tasks = [process_batch(batch) for batch in batched_data]

        # Process all batches and yield results as they complete
        results: list[Vector] = []
        for task in asyncio.as_completed(tasks):
            results.extend(await task)
        return results
