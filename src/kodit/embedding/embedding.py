"""Embedding service."""

import asyncio
import os
from abc import ABC, abstractmethod
from collections.abc import AsyncGenerator
from typing import NamedTuple

import structlog
import tiktoken
from openai import AsyncOpenAI
from sentence_transformers import SentenceTransformer

TINY = "tiny"
CODE = "code"
TEST = "test"

COMMON_EMBEDDING_MODELS = {
    TINY: "ibm-granite/granite-embedding-30m-english",
    CODE: "flax-sentence-embeddings/st-codesearch-distilroberta-base",
    TEST: "minishlab/potion-base-4M",
}


class EmbeddingInput(NamedTuple):
    """Input for embedding."""

    id: int
    text: str


class EmbeddingOutput(NamedTuple):
    """Output for embedding."""

    id: int
    embedding: list[float]


class Embedder(ABC):
    """Embedder interface."""

    @abstractmethod
    def embed(
        self, data: list[EmbeddingInput]
    ) -> AsyncGenerator[EmbeddingOutput, None]:
        """Embed a list of documents.

        The embedding service accepts a massive list of id,strings to embed. Behind the
        scenes it batches up requests and parallelizes them for performance according to
        the specifics of the embedding service.

        The id reference is required because the parallelization may return results out
        of order.
        """

    @abstractmethod
    def query(self, data: list[str]) -> AsyncGenerator[list[float], None]:
        """Query the embedding model."""


def embedding_factory(openai_client: AsyncOpenAI | None = None) -> Embedder:
    """Create an embedding service."""
    if openai_client is not None:
        return OpenAIEmbedder(openai_client)
    return LocalEmbedder(model_name=TINY)


class LocalEmbedder(Embedder):
    """Local embedder."""

    def __init__(self, model_name: str) -> None:
        """Initialize the local embedder."""
        self.log = structlog.get_logger(__name__)
        self.log.info("Creating local embedder", model_name=model_name)
        self.model_name = COMMON_EMBEDDING_MODELS.get(model_name, model_name)
        self.embedding_model = None
        self.encoding = tiktoken.encoding_for_model("text-embedding-3-small")

    def _model(self) -> SentenceTransformer:
        """Get the embedding model."""
        if self.embedding_model is None:
            os.environ["TOKENIZERS_PARALLELISM"] = "false"  # Avoid warnings
            self.embedding_model = SentenceTransformer(
                self.model_name,
                trust_remote_code=True,
                device="cpu",  # Force CPU so we don't have to install accelerate, etc.
            )
        return self.embedding_model

    async def embed(
        self, data: list[EmbeddingInput]
    ) -> AsyncGenerator[EmbeddingOutput, None]:
        """Embed a list of documents."""
        model = self._model()

        batched_data = _split_sub_batches(self.encoding, data)

        for batch in batched_data:
            embeddings = model.encode(
                [i.text for i in batch], show_progress_bar=False, batch_size=4
            )
            for i, x in zip(batch, embeddings, strict=False):
                yield EmbeddingOutput(i.id, [float(y) for y in x])

    async def query(self, data: list[str]) -> AsyncGenerator[list[float], None]:
        """Query the embedding model."""
        model = self._model()
        embeddings = model.encode(data, show_progress_bar=False, batch_size=4)
        for embedding in embeddings:
            yield [float(x) for x in embedding]


OPENAI_MAX_EMBEDDING_SIZE = 8192
OPENAI_NUM_PARALLEL_TASKS = 10


def _split_sub_batches(
    encoding: tiktoken.Encoding, data: list[EmbeddingInput]
) -> list[list[EmbeddingInput]]:
    """Split a list of strings into smaller sub-batches."""
    log = structlog.get_logger(__name__)
    result = []
    data_to_process = [s for s in data if s.text.strip()]  # Filter out empty strings

    while data_to_process:
        next_batch = []
        current_tokens = 0

        while data_to_process:
            next_item = data_to_process[0]
            item_tokens = len(encoding.encode(next_item.text))

            if item_tokens > OPENAI_MAX_EMBEDDING_SIZE:
                log.warning("Skipping too long snippet", snippet=data_to_process.pop(0))
                continue

            if current_tokens + item_tokens > OPENAI_MAX_EMBEDDING_SIZE:
                break

            next_batch.append(data_to_process.pop(0))
            current_tokens += item_tokens

        if next_batch:
            result.append(next_batch)

    return result


class OpenAIEmbedder(Embedder):
    """OpenAI embedder."""

    def __init__(
        self, openai_client: AsyncOpenAI, model_name: str = "text-embedding-3-small"
    ) -> None:
        """Initialize the OpenAI embedder."""
        self.log = structlog.get_logger(__name__)
        self.log.info("Creating OpenAI embedder", model_name=model_name)
        self.openai_client = openai_client
        self.encoding = tiktoken.encoding_for_model(model_name)
        self.log = structlog.get_logger(__name__)

    async def embed(
        self,
        data: list[EmbeddingInput],
    ) -> AsyncGenerator[EmbeddingOutput, None]:
        """Embed a list of documents."""
        # First split the list into a list of list where each sublist has fewer than
        # max tokens.
        batched_data = _split_sub_batches(self.encoding, data)

        # Process batches in parallel with a semaphore to limit concurrent requests
        sem = asyncio.Semaphore(OPENAI_NUM_PARALLEL_TASKS)

        async def process_batch(batch: list[EmbeddingInput]) -> list[EmbeddingOutput]:
            async with sem:
                try:
                    response = await self.openai_client.embeddings.create(
                        model="text-embedding-3-small",
                        input=[i.text for i in batch],
                    )
                    return [
                        EmbeddingOutput(i.id, x.embedding)
                        for i, x in zip(batch, response.data, strict=False)
                    ]
                except Exception as e:
                    self.log.exception("Error embedding batch", error=str(e))
                    return []

        # Create tasks for all batches
        tasks = [process_batch(batch) for batch in batched_data]

        # Process all batches and yield results as they complete
        for task in asyncio.as_completed(tasks):
            embeddings = await task
            for e in embeddings:
                yield e

    async def query(self, data: list[str]) -> AsyncGenerator[list[float], None]:
        """Query the embedding model."""
        async for e in self.embed(
            [EmbeddingInput(i, text) for i, text in enumerate(data)]
        ):
            yield e.embedding
