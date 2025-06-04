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

        # Create a list of tuples with a temporary id for each batch
        # We need to do this so that we can return the results in the same order as the
        # input data
        input_data = [(i, batch) for i, batch in enumerate(batched_data)]

        async def process_batch(
            data: tuple[int, list[str]],
        ) -> tuple[int, list[Vector]]:
            batch_id, batch = data
            async with sem:
                try:
                    response = await self.openai_client.embeddings.create(
                        model=self.model_name,
                        input=batch,
                    )
                    return batch_id, [
                        [float(x) for x in embedding.embedding]
                        for embedding in response.data
                    ]
                except Exception as e:
                    self.log.exception("Error embedding batch", error=str(e))
                    return batch_id, []

        # Create tasks for all batches
        tasks = [process_batch(batch) for batch in input_data]

        # Process all batches and yield results as they complete
        results: list[tuple[int, list[Vector]]] = []
        for task in asyncio.as_completed(tasks):
            result = await task
            results.append(result)

        # Output in the same order as the input data
        ordered_results = [result for _, result in sorted(results, key=lambda x: x[0])]
        return [item for sublist in ordered_results for item in sublist]
