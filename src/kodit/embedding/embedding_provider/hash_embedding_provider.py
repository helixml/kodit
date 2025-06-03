"""Hash embedding provider, for use in tests only."""

import asyncio
import hashlib
import math
from collections.abc import Generator, Sequence

from kodit.embedding.embedding_provider.embedding_provider import (
    EmbeddingProvider,
    Vector,
)


class HashEmbeddingProvider(EmbeddingProvider):
    """A minimal test-time embedding provider.

    • Zero third-party dependencies (uses only std-lib)
    • Distinguishes strings by hashing with SHA-256
    • Maps the digest to a fixed-size float vector, then ℓ₂-normalises
    • Splits work into small asynchronous chunks for speed in event loops
    """

    def __init__(self, dim: int = 16, batch_size: int = 64) -> None:
        """Initialize the hash embedding provider."""
        if dim <= 0:
            msg = f"dim must be > 0, got {dim}"
            raise ValueError(msg)
        if batch_size <= 0:
            msg = f"batch_size must be > 0, got {batch_size}"
            raise ValueError(msg)
        self.dim = dim
        self.batch_size = batch_size

    async def embed(self, data: list[str]) -> list[Vector]:
        """Embed every string in *data*, preserving order.

        Work is sliced into *batch_size* chunks and scheduled concurrently
        (still CPU-bound, but enough to cooperate with an asyncio loop).
        """
        if not data:
            return []

        async def _embed_chunk(chunk: Sequence[str]) -> list[Vector]:
            return [self._string_to_vector(text) for text in chunk]

        tasks = [
            asyncio.create_task(_embed_chunk(chunk))
            for chunk in self._chunked(data, self.batch_size)
        ]

        vectors: list[Vector] = []
        for task in tasks:
            vectors.extend(await task)
        return vectors

    @staticmethod
    def _chunked(seq: Sequence[str], size: int) -> Generator[Sequence[str], None, None]:
        """Yield successive *size*-sized slices from *seq*."""
        for i in range(0, len(seq), size):
            yield seq[i : i + size]

    def _string_to_vector(self, text: str) -> Vector:
        """Deterministically convert *text* to a normalised float vector."""
        digest = hashlib.sha256(text.encode("utf-8")).digest()

        # Build the vector from 4-byte windows of the digest.
        vec = [
            int.from_bytes(
                digest[(i * 4) % len(digest) : (i * 4) % len(digest) + 4], "big"
            )
            / 0xFFFFFFFF
            for i in range(self.dim)
        ]

        # ℓ₂-normalise so magnitudes are comparable.
        norm = math.sqrt(sum(x * x for x in vec)) or 1.0
        return [x / norm for x in vec]
