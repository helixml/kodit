"""LiteLLM embedding provider implementation."""

import asyncio
from collections.abc import AsyncGenerator
from typing import Any

import httpx
import litellm
import structlog
import tiktoken
from litellm import aembedding

from kodit.config import Endpoint
from kodit.domain.services.embedding_service import EmbeddingProvider
from kodit.domain.value_objects import EmbeddingRequest, EmbeddingResponse
from kodit.infrastructure.embedding.embedding_providers.batching import (
    split_sub_batches,
)


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
        self.endpoint = endpoint
        self.log = structlog.get_logger(__name__)
        self._encoding: tiktoken.Encoding | None = None

        # Configure LiteLLM with custom HTTPX client for Unix socket support if needed
        self._setup_litellm_client()

    def _setup_litellm_client(self) -> None:
        """Set up LiteLLM with custom HTTPX client for Unix socket support."""
        if self.endpoint.socket_path:
            # Create HTTPX client with Unix socket transport
            transport = httpx.AsyncHTTPTransport(uds=self.endpoint.socket_path)
            unix_client = httpx.AsyncClient(
                transport=transport,
                base_url="http://localhost",  # Base URL for Unix socket
                timeout=self.endpoint.timeout,
            )
            # Set as LiteLLM's async client session
            litellm.aclient_session = unix_client

    def _split_sub_batches(
        self, encoding: tiktoken.Encoding, data: list[EmbeddingRequest]
    ) -> list[list[EmbeddingRequest]]:
        """Proxy to the shared batching utility (kept for backward-compat)."""
        return split_sub_batches(
            encoding,
            data,
            max_tokens=self.endpoint.max_tokens,
            batch_size=self.endpoint.num_parallel_tasks,
        )

    async def _call_embeddings_api(self, texts: list[str]) -> Any:
        """Call the embeddings API using LiteLLM.

        Args:
            texts: The texts to embed.

        Returns:
            The API response as a dictionary.

        """
        kwargs = {
            "model": self.endpoint.model,
            "input": texts,
            "timeout": self.endpoint.timeout,
        }

        # Add API key if provided
        if self.endpoint.api_key:
            kwargs["api_key"] = self.endpoint.api_key

        # Add base_url if provided
        if self.endpoint.base_url:
            kwargs["api_base"] = self.endpoint.base_url

        # Add extra parameters
        kwargs.update(self.endpoint.extra_params or {})

        try:
            # Use litellm's async embedding function
            response = await aembedding(**kwargs)
            return (
                response.model_dump() if hasattr(response, "model_dump") else response
            )
        except Exception as e:
            self.log.exception(
                "LiteLLM embedding API error", error=str(e), model=self.endpoint.model
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
        encoding = self._get_encoding()
        batched_data = self._split_sub_batches(encoding, data)

        # Process batches concurrently with semaphore
        sem = asyncio.Semaphore(self.endpoint.num_parallel_tasks or 10)

        async def _process_batch(
            batch: list[EmbeddingRequest],
        ) -> list[EmbeddingResponse]:
            async with sem:
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

        tasks = [_process_batch(batch) for batch in batched_data]
        for task in asyncio.as_completed(tasks):
            yield await task

    async def close(self) -> None:
        """Close the provider and cleanup HTTPX client if using Unix sockets."""
        if (
            self.endpoint.socket_path
            and hasattr(litellm, "aclient_session")
            and litellm.aclient_session
        ):
            await litellm.aclient_session.aclose()
            litellm.aclient_session = None

    def _get_encoding(self) -> tiktoken.Encoding:
        """Return (and cache) the tiktoken encoding for the chosen model."""
        if self._encoding is None:
            self._encoding = tiktoken.get_encoding(
                "o200k_base"
            )  # Reasonable default for most models, but might not be perfect.
        return self._encoding
