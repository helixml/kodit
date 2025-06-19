"""OpenAI embedding provider implementation."""

from collections.abc import AsyncGenerator

import structlog

from kodit.domain.models import EmbeddingRequest, EmbeddingResponse
from kodit.domain.services.embedding_service import EmbeddingProvider


class OpenAIEmbeddingProvider(EmbeddingProvider):
    """OpenAI embedding provider that uses OpenAI's embedding API."""

    def __init__(
        self, openai_client, model_name: str = "text-embedding-3-small"
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

            async def empty_generator():
                if False:
                    yield []

            return empty_generator()

        # Process in batches
        batch_size = 10

        async def _embed_batches():
            for i in range(0, len(data), batch_size):
                batch = data[i : i + batch_size]
                texts = [request.text for request in batch]

                try:
                    # Call OpenAI API
                    response = await self.openai_client.embeddings.create(
                        model=self.model_name,
                        input=texts,
                    )

                    # Convert response to domain models
                    responses = []
                    for j, embedding_data in enumerate(response.data):
                        responses.append(
                            EmbeddingResponse(
                                id=batch[j].id, embedding=embedding_data.embedding
                            )
                        )

                    yield responses

                except Exception as e:
                    self.log.error("Error calling OpenAI API", error=str(e))
                    # Return empty embeddings on error
                    responses = [
                        EmbeddingResponse(id=request.id, embedding=[0.0] * 1536)
                        for request in batch
                    ]
                    yield responses

        return _embed_batches()
