"""OpenAI embedding service."""

import asyncio
from collections.abc import AsyncGenerator

import structlog
import tiktoken
from openai import AsyncOpenAI

from kodit.embedding.embedding_provider.embedding_provider import (
    EmbeddingProvider,
    EmbeddingRequest,
    EmbeddingResponse,
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
        self.encoding = tiktoken.encoding_for_model(
            "text-embedding-3-small"
        )  # Sensible default

    async def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of documents."""
        # First split the list into a list of list where each sublist has fewer than
        # max tokens.
        batched_data = split_sub_batches(self.encoding, data)

        # Process batches in parallel with a semaphore to limit concurrent requests
        sem = asyncio.Semaphore(OPENAI_NUM_PARALLEL_TASKS)

        async def process_batch(
            data: list[EmbeddingRequest],
        ) -> list[EmbeddingResponse]:
            async with sem:
                try:
                    response = await self.openai_client.embeddings.create(
                        model=self.model_name,
                        input=[i.text for i in data],
                    )
                    return [
                        EmbeddingResponse(
                            id=item.id,
                            embedding=embedding.embedding,
                        )
                        for item, embedding in zip(data, response.data, strict=True)
                    ]
                except Exception as e:
                    self.log.exception("Error embedding batch", error=str(e))
                    return []

        # Create tasks for all batches
        tasks = [process_batch(batch) for batch in batched_data]

        # Process all batches and yield results as they complete
        for task in asyncio.as_completed(tasks):
            result = await task
            yield result
