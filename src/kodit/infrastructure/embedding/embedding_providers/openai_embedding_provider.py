"""OpenAI embedding provider implementation using httpx."""

import asyncio
from collections.abc import AsyncGenerator
from typing import Any

import httpx
import structlog
import tiktoken
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
    """OpenAI embedding provider that uses OpenAI's embedding API via httpx."""

    def __init__(  # noqa: PLR0913
        self,
        api_key: str | None = None,
        base_url: str = "https://api.openai.com",
        model_name: str = "text-embedding-3-small",
        num_parallel_tasks: int = OPENAI_NUM_PARALLEL_TASKS,
        socket_path: str | None = None,
        timeout: float = 30.0,
    ) -> None:
        """Initialize the OpenAI embedding provider.

        Args:
            api_key: The OpenAI API key.
            base_url: The base URL for the OpenAI API.
            model_name: The model name to use for embeddings.
            num_parallel_tasks: Maximum number of concurrent requests.
            socket_path: Optional Unix socket path for local communication.
            timeout: Request timeout in seconds.

        """
        self.model_name = model_name
        self.num_parallel_tasks = num_parallel_tasks
        self.log = structlog.get_logger(__name__)
        self.api_key = api_key
        self.base_url = base_url
        self.socket_path = socket_path
        self.timeout = timeout

        # Lazily initialised token encoding
        self._encoding: Encoding | None = None

        # Create httpx client with optional Unix socket support
        if socket_path:
            transport = httpx.AsyncHTTPTransport(uds=socket_path)
            self.http_client = httpx.AsyncClient(
                transport=transport,
                base_url="http://localhost",  # Base URL for Unix socket
                timeout=timeout,
            )
        else:
            self.http_client = httpx.AsyncClient(
                base_url=base_url,
                timeout=timeout,
            )

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

    async def _call_embeddings_api(
        self, texts: list[str]
    ) -> dict[str, Any]:
        """Call the embeddings API using httpx.

        Args:
            texts: The texts to embed.

        Returns:
            The API response as a dictionary.

        """
        headers = {
            "Content-Type": "application/json",
        }
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        data = {
            "model": self.model_name,
            "input": texts,
        }

        response = await self.http_client.post(
            "/v1/embeddings",
            json=data,
            headers=headers,
        )
        response.raise_for_status()
        return response.json()

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
                    response = await self._call_embeddings_api(
                        [item.text for item in batch]
                    )
                    embeddings_data = response.get("data", [])

                    return [
                        EmbeddingResponse(
                            snippet_id=item.snippet_id,
                            embedding=emb_data.get("embedding", []),
                        )
                        for item, emb_data in zip(batch, embeddings_data, strict=True)
                    ]
                except Exception as e:
                    self.log.exception("Error embedding batch", error=str(e))
                    # Return no embeddings for this batch if there was an error
                    return []

        tasks = [_process_batch(batch) for batch in batched_data]
        for task in asyncio.as_completed(tasks):
            yield await task

    async def close(self) -> None:
        """Close the HTTP client."""
        if hasattr(self, "http_client"):
            await self.http_client.aclose()

