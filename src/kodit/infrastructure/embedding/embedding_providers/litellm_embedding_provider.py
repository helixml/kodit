"""LiteLLM embedding provider implementation."""

from collections.abc import AsyncGenerator

import structlog
import tiktoken

from kodit.config import Endpoint
from kodit.domain.services.embedding_service import EmbeddingProvider
from kodit.domain.value_objects import EmbeddingRequest, EmbeddingResponse
from kodit.infrastructure.embedding.embedding_providers.batching import (
    split_sub_batches,
)
from kodit.infrastructure.providers.async_batch_processor import (
    process_items_concurrently,
)
from kodit.infrastructure.providers.litellm_provider import LiteLLMProvider


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
        self.provider: LiteLLMProvider = LiteLLMProvider(self.endpoint)

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

        async def _process_batch(
            batch: list[EmbeddingRequest],
        ) -> list[EmbeddingResponse]:
            texts = [item.text for item in batch]
            response = await self.provider.embedding(texts)
            embeddings_data = response.get("data", [])

            # Handle mismatch between batch size and response size
            if len(embeddings_data) != len(batch):
                preview_response = embeddings_data[:3] if embeddings_data else None
                self.log.error(
                    "Embedding response size mismatch",
                    batch_size=len(batch),
                    response_size=len(embeddings_data),
                    texts_preview=[t[:50] for t in texts[:3]],
                    response_preview=preview_response,
                )
                raise ValueError(
                    f"Expected {len(batch)} embeddings, got {len(embeddings_data)}"
                )

            return [
                EmbeddingResponse(
                    snippet_id=item.snippet_id,
                    embedding=emb_data.get("embedding", []),
                )
                for item, emb_data in zip(batch, embeddings_data, strict=True)
            ]

        async for result in process_items_concurrently(
            batched_data,
            _process_batch,
            self.endpoint.num_parallel_tasks,
        ):
            yield result

    async def close(self) -> None:
        """Close the provider."""
        await self.provider.close()

    def _get_encoding(self) -> tiktoken.Encoding:
        """Return (and cache) the tiktoken encoding for the chosen model."""
        if self._encoding is None:
            self._encoding = tiktoken.get_encoding(
                "o200k_base"
            )  # Reasonable default for most models, but might not be perfect.
        return self._encoding
