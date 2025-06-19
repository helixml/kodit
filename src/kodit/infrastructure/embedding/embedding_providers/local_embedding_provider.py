"""Local embedding provider implementation."""

import hashlib
from collections.abc import AsyncGenerator

import structlog

from kodit.domain.models import EmbeddingRequest, EmbeddingResponse
from kodit.domain.services.embedding_service import EmbeddingProvider

# Constants for different embedding sizes
TINY = 64
CODE = 1536


class LocalEmbeddingProvider(EmbeddingProvider):
    """Local embedding provider that generates deterministic embeddings."""

    def __init__(self, embedding_size: int = CODE) -> None:
        """Initialize the local embedding provider.

        Args:
            embedding_size: The size of the embedding vectors to generate

        """
        self.embedding_size = embedding_size
        self.log = structlog.get_logger(__name__)

    def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of strings using a simple hash-based approach."""
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
                responses = []

                for request in batch:
                    # Generate a deterministic embedding based on the text
                    embedding = self._generate_embedding(request.text)
                    responses.append(
                        EmbeddingResponse(id=request.id, embedding=embedding)
                    )

                yield responses

        return _embed_batches()

    def _generate_embedding(self, text: str) -> list[float]:
        """Generate a deterministic embedding for the given text."""
        # Use SHA-256 hash of the text as a seed
        hash_obj = hashlib.sha256(text.encode("utf-8"))
        hash_bytes = hash_obj.digest()

        # Convert hash bytes to a list of floats
        embedding = []
        for i in range(self.embedding_size):
            # Use different bytes for each dimension
            byte_index = i % len(hash_bytes)
            # Convert byte to float between -1 and 1
            value = (hash_bytes[byte_index] - 128) / 128.0
            embedding.append(value)

        return embedding
