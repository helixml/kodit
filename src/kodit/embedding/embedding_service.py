"""Embedding service."""

import asyncio
import os
from abc import ABC, abstractmethod
from typing import NamedTuple

import structlog
import tiktoken
from openai import AsyncOpenAI
from sentence_transformers import SentenceTransformer
from tqdm import tqdm

from kodit.embedding.embedding_models import Embedding, EmbeddingType
from kodit.embedding.embedding_repository import EmbeddingRepository

TINY = "tiny"
CODE = "code"
TEST = "test"

COMMON_EMBEDDING_MODELS = {
    TINY: "ibm-granite/granite-embedding-30m-english",
    CODE: "flax-sentence-embeddings/st-codesearch-distilroberta-base",
    TEST: "minishlab/potion-base-4M",
}


class EmbeddingResult(NamedTuple):
    """Embedding result."""

    snippet_id: int
    score: float


class EmbeddingInput(NamedTuple):
    """Input for embedding."""

    snippet_id: int
    text: str


class EmbeddingService(ABC):
    """Embedder interface."""

    @abstractmethod
    async def index(self, data: list[EmbeddingInput]) -> None:
        """Embed a list of documents.

        The embedding service accepts a massive list of id,strings to embed. Behind the
        scenes it batches up requests and parallelizes them for performance according to
        the specifics of the embedding service.

        The id reference is required because the parallelization may return results out
        of order.
        """

    @abstractmethod
    async def retrieve(self, query: str, top_k: int = 10) -> list[EmbeddingResult]:
        """Query the embedding model."""


class LocalEmbedder(EmbeddingService):
    """Local embedder."""

    def __init__(
        self, embedding_repository: EmbeddingRepository, model_name: str
    ) -> None:
        """Initialize the local embedder."""
        self.log = structlog.get_logger(__name__)
        self.log.info("Creating local embedder", model_name=model_name)
        self.model_name = COMMON_EMBEDDING_MODELS.get(model_name, model_name)
        self.embedding_model = None
        self.encoding = tiktoken.encoding_for_model("text-embedding-3-small")
        self.embedding_repository = embedding_repository

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

    async def index(self, data: list[EmbeddingInput]) -> None:
        """Embed a list of documents."""
        model = self._model()

        batched_data = _split_sub_batches(self.encoding, data)

        self.log.info("Embedding snippets", num_snippets=len(data))
        for batch in tqdm(batched_data, total=len(batched_data), leave=False):
            embeddings = model.encode(
                [i.text for i in batch], show_progress_bar=False, batch_size=4
            )
            for i, x in zip(batch, embeddings, strict=False):
                await self.embedding_repository.create_embedding(
                    Embedding(
                        snippet_id=i.snippet_id,
                        embedding=[float(y) for y in x],
                        type=EmbeddingType.CODE,
                    )
                )

    async def retrieve(self, query: str, top_k: int = 10) -> list[EmbeddingResult]:
        """Query the embedding model."""
        model = self._model()
        embedding = model.encode(query, show_progress_bar=False, batch_size=4)
        results = await self.embedding_repository.list_semantic_results(
            EmbeddingType.CODE, [float(x) for x in embedding], top_k
        )
        return [EmbeddingResult(snippet_id, score) for snippet_id, score in results]


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


class OpenAIEmbedder(EmbeddingService):
    """OpenAI embedder."""

    def __init__(
        self,
        embedding_repository: EmbeddingRepository,
        openai_client: AsyncOpenAI,
        model_name: str = "text-embedding-3-small",
    ) -> None:
        """Initialize the OpenAI embedder."""
        self.log = structlog.get_logger(__name__)
        self.log.info("Creating OpenAI embedder", model_name=model_name)
        self.openai_client = openai_client
        self.encoding = tiktoken.encoding_for_model(model_name)
        self.log = structlog.get_logger(__name__)
        self.embedding_repository = embedding_repository

    async def index(self, data: list[EmbeddingInput]) -> None:
        """Embed a list of documents."""
        # First split the list into a list of list where each sublist has fewer than
        # max tokens.
        batched_data = _split_sub_batches(self.encoding, data)

        # Process batches in parallel with a semaphore to limit concurrent requests
        sem = asyncio.Semaphore(OPENAI_NUM_PARALLEL_TASKS)

        async def process_batch(batch: list[EmbeddingInput]) -> list[Embedding]:
            async with sem:
                try:
                    response = await self.openai_client.embeddings.create(
                        model="text-embedding-3-small",
                        input=[i.text for i in batch],
                    )
                    return [
                        Embedding(
                            snippet_id=i.snippet_id,
                            embedding=[float(x) for x in x.embedding],
                            type=EmbeddingType.CODE,
                        )
                        for i, x in zip(batch, response.data, strict=False)
                    ]
                except Exception as e:
                    self.log.exception("Error embedding batch", error=str(e))
                    return []

        # Create tasks for all batches
        tasks = [process_batch(batch) for batch in batched_data]

        # Process all batches and yield results as they complete
        self.log.info("Embedding snippets", num_snippets=len(data))
        for task in tqdm(asyncio.as_completed(tasks), total=len(tasks), leave=False):
            embeddings = await task
            for e in embeddings:
                await self.embedding_repository.create_embedding(e)

    async def retrieve(self, query: str, top_k: int = 10) -> list[EmbeddingResult]:
        """Query the embedding model."""
        embedding = await self.openai_client.embeddings.create(
            model="text-embedding-3-small",
            input=[query],
        )
        results = await self.embedding_repository.list_semantic_results(
            EmbeddingType.CODE, [float(x) for x in embedding.data[0].embedding], top_k
        )
        return [EmbeddingResult(snippet_id, score) for snippet_id, score in results]
