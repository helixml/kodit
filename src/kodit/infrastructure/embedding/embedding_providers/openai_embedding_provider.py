"""OpenAI embedding provider implementation."""

import asyncio
from collections.abc import AsyncGenerator

import structlog
import tiktoken
from openai import AsyncOpenAI
from tiktoken import Encoding

from kodit.domain.services.embedding_service import EmbeddingProvider
from kodit.domain.value_objects import EmbeddingRequest, EmbeddingResponse

from .batching import split_sub_batches

# Constants
MAX_TOKENS = 8192  # Conservative token limit for the embedding model
BATCH_SIZE = (
    10  # Maximum number of items per API call (keeps existing test expectations)
)
OPENAI_NUM_PARALLEL_TASKS = 10  # Semaphore limit for concurrent OpenAI requests


class OpenAIEmbeddingProvider(EmbeddingProvider):
    """OpenAI embedding provider that uses OpenAI's embedding API."""

    def __init__(
        self,
        openai_client: AsyncOpenAI,
        model_name: str = "text-embedding-3-small",
        num_parallel_tasks: int = OPENAI_NUM_PARALLEL_TASKS,
    ) -> None:
        """Initialize the OpenAI embedding provider.

        Args:
            openai_client: The OpenAI client instance
            model_name: The model name to use for embeddings

        """
        self.openai_client = openai_client
        self.model_name = model_name
        self.num_parallel_tasks = num_parallel_tasks
        self.log = structlog.get_logger(__name__)

        # Lazily initialised token encoding
        self._encoding: Encoding | None = None

    # ---------------------------------------------------------------------
    # Helper utilities
    # ---------------------------------------------------------------------

    def _get_encoding(self) -> "Encoding":
        """Return (and cache) the tiktoken encoding for the chosen model."""
        if self._encoding is None:
            try:
                self._encoding = tiktoken.encoding_for_model(self.model_name)
            except KeyError:
                # If the model is not supported by tiktoken, use a default encoding
                self.log.info(
                    "Model not supported by tiktoken, using default encoding",
                    model_name=self.model_name,
                    default_encoding="o200k_base",
                )
                self._encoding = tiktoken.get_encoding("o200k_base")

        return self._encoding

    def _split_sub_batches(
        self, encoding: "Encoding", data: list[EmbeddingRequest]
    ) -> list[list[EmbeddingRequest]]:
        """Proxy to the shared batching utility (kept for backward-compat)."""
        return split_sub_batches(
            encoding,
            data,
            max_tokens=MAX_TOKENS,
            batch_size=BATCH_SIZE,
        )

    async def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of strings using OpenAI's API."""
        if not data:
            yield []

        encoding = self._get_encoding()

        # First, split by token limits (and max batch size)
        batched_data = self._split_sub_batches(encoding, data)

        # -----------------------------------------------------------------
        # Process batches concurrently (but bounded by a semaphore)
        # -----------------------------------------------------------------

        sem = asyncio.Semaphore(self.num_parallel_tasks)

        async def _process_batch(
            batch: list[EmbeddingRequest],
        ) -> list[EmbeddingResponse]:
            async with sem:
                try:
                    response = await self.openai_client.embeddings.create(
                        model=self.model_name,
                        input=[item.text for item in batch],
                    )

                    return [
                        EmbeddingResponse(
                            snippet_id=item.snippet_id,
                            embedding=embedding.embedding,
                        )
                        for item, embedding in zip(batch, response.data, strict=True)
                    ]
                except Exception as e:
                    self.log.exception("Error embedding batch", error=str(e))
                    # Return no embeddings for this batch if there was an error
                    return []

        tasks = [_process_batch(batch) for batch in batched_data]
        for task in asyncio.as_completed(tasks):
            yield await task
