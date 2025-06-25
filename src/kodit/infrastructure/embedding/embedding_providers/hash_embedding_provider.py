"""Hash-based embedding provider for testing purposes."""

import hashlib
from collections.abc import AsyncGenerator

import structlog

from kodit.domain.services.embedding_service import EmbeddingProvider
from kodit.domain.value_objects import EmbeddingRequest, EmbeddingResponse

# Constants for different embedding sizes
TINY = 64
CODE = 1536


class HashEmbeddingProvider(EmbeddingProvider):
    """Hash-based embedding that generates deterministic embeddings for testing."""

    def __init__(self, embedding_size: int = CODE) -> None:
        """Initialize the hash embedding provider.

        Args:
            embedding_size: The size of the embedding vectors to generate

        """
        self.embedding_size = embedding_size
        self.log = structlog.get_logger(__name__)

    async def embed(
        self, data: list[EmbeddingRequest]
    ) -> AsyncGenerator[list[EmbeddingResponse], None]:
        """Embed a list of strings using a simple hash-based approach."""
        if not data:
            yield []

        # Process in batches
        batch_size = 10
        for i in range(0, len(data), batch_size):
            batch = data[i : i + batch_size]
            responses = []

            for request in batch:
                # Generate a deterministic embedding based on the text
                embedding = self._generate_embedding(request.text)
                responses.append(
                    EmbeddingResponse(
                        snippet_id=request.snippet_id, embedding=embedding
                    )
                )

            yield responses

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
