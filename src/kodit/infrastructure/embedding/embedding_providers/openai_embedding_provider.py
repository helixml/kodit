"""OpenAI embedding provider implementation."""

from collections.abc import AsyncGenerator
from typing import Any

import structlog

from kodit.domain.services.embedding_service import EmbeddingProvider
from kodit.domain.value_objects import EmbeddingRequest, EmbeddingResponse


class OpenAIEmbeddingProvider(EmbeddingProvider):
    """OpenAI embedding provider that uses OpenAI's embedding API."""

    def __init__(
        self, openai_client: Any, model_name: str = "text-embedding-3-small"
    ) -> None:
        """Initialize the OpenAI embedding provider.

        Args:
            openai_client: The OpenAI client instance
            model_name: The model name to use for embeddings

        """
        self.openai_client = openai_client
        self.model_name = model_name
        self.log = structlog.get_logger(__name__)

    def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of strings using OpenAI's API."""
        if not data:

            async def empty_generator() -> AsyncGenerator[
                list[EmbeddingResponse], None
            ]:
                if False:
                    yield []

            return empty_generator()

        # Process in batches
        batch_size = 10

        async def _embed_batches() -> AsyncGenerator[list[EmbeddingResponse], None]:
            for i in range(0, len(data), batch_size):
                batch = data[i : i + batch_size]
                try:
                    # Prepare the texts for embedding
                    texts = [request.text for request in batch]

                    # Call OpenAI API
                    response = await self.openai_client.embeddings.create(
                        input=texts, model=self.model_name
                    )

                    # Convert response to our format
                    responses = []
                    for j, embedding_data in enumerate(response.data):
                        responses.append(
                            EmbeddingResponse(
                                snippet_id=batch[j].snippet_id,
                                embedding=embedding_data.embedding,
                            )
                        )

                    yield responses

                except Exception:
                    self.log.exception("Error calling OpenAI API")
                    # Return empty embeddings on error
                    responses = [
                        EmbeddingResponse(
                            snippet_id=request.snippet_id,
                            embedding=[0.0] * 1536,  # Default embedding size
                        )
                        for request in batch
                    ]
                    yield responses

        return _embed_batches()
