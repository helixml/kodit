"""LiteLLM embedding provider implementation."""

import asyncio
from collections.abc import AsyncGenerator
from typing import Any

import httpx
import litellm
import structlog
from litellm import aembedding

from kodit.config import Endpoint
from kodit.domain.services.embedding_service import EmbeddingProvider
from kodit.domain.value_objects import EmbeddingRequest, EmbeddingResponse

# Constants
MAX_TOKENS = 8192  # Conservative token limit for the embedding model
BATCH_SIZE = 10  # Maximum number of items per API call
DEFAULT_NUM_PARALLEL_TASKS = 10  # Semaphore limit for concurrent requests


class LiteLLMEmbeddingProvider(EmbeddingProvider):
    """LiteLLM embedding provider that supports 100+ providers."""

    def __init__(
        self,
        endpoint: Endpoint,
    ) -> None:
        """Initialize the LiteLLM embedding provider.

        Args:
            endpoint: The endpoint configuration containing all settings.

        """
        self.model_name = endpoint.model or "text-embedding-3-small"
        self.api_key = endpoint.api_key
        self.base_url = endpoint.base_url
        self.socket_path = endpoint.socket_path
        self.num_parallel_tasks = (
            endpoint.num_parallel_tasks or DEFAULT_NUM_PARALLEL_TASKS
        )
        self.timeout = endpoint.timeout or 30.0
        self.extra_params = endpoint.extra_params or {}
        self.log = structlog.get_logger(__name__)

        # Configure LiteLLM with custom HTTPX client for Unix socket support if needed
        self._setup_litellm_client()

    def _setup_litellm_client(self) -> None:
        """Set up LiteLLM with custom HTTPX client for Unix socket support."""
        if self.socket_path:
            # Create HTTPX client with Unix socket transport
            transport = httpx.AsyncHTTPTransport(uds=self.socket_path)
            unix_client = httpx.AsyncClient(
                transport=transport,
                base_url="http://localhost",  # Base URL for Unix socket
                timeout=self.timeout,
            )
            # Set as LiteLLM's async client session
            litellm.aclient_session = unix_client

    def _split_sub_batches(
        self, data: list[EmbeddingRequest]
    ) -> list[list[EmbeddingRequest]]:
        """Split data into manageable batches.

        For LiteLLM, we use a simpler batching approach since token counting
        varies by provider. We use a conservative batch size approach.
        """
        batches = []
        for i in range(0, len(data), BATCH_SIZE):
            batch = data[i : i + BATCH_SIZE]
            batches.append(batch)
        return batches

    async def _call_embeddings_api(self, texts: list[str]) -> Any:
        """Call the embeddings API using LiteLLM.

        Args:
            texts: The texts to embed.

        Returns:
            The API response as a dictionary.

        """
        kwargs = {
            "model": self.model_name,
            "input": texts,
            "timeout": self.timeout,
        }

        # Add API key if provided
        if self.api_key:
            kwargs["api_key"] = self.api_key

        # Add base_url if provided
        if self.base_url:
            kwargs["api_base"] = self.base_url

        # Add extra parameters
        kwargs.update(self.extra_params)

        try:
            # Use litellm's async embedding function
            response = await aembedding(**kwargs)
            return (
                response.model_dump() if hasattr(response, "model_dump") else response
            )
        except Exception as e:
            self.log.exception(
                "LiteLLM embedding API error", error=str(e), model=self.model_name
            )
            raise

    async def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of strings using LiteLLM."""
        if not data:
            yield []
            return

        # Split into batches
        batched_data = self._split_sub_batches(data)

        # Process batches concurrently with semaphore
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
        """Close the provider and cleanup HTTPX client if using Unix sockets."""
        if (
            self.socket_path
            and hasattr(litellm, "aclient_session")
            and litellm.aclient_session
        ):
            await litellm.aclient_session.aclose()
            litellm.aclient_session = None
