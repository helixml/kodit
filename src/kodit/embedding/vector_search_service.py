"""Embedding service."""

from abc import ABC, abstractmethod
from typing import NamedTuple


class VectorSearchResponse(NamedTuple):
    """Embedding result."""

    snippet_id: int
    score: float


class VectorSearchRequest(NamedTuple):
    """Input for embedding."""

    snippet_id: int
    text: str


class VectorSearchService(ABC):
    """Semantic search service interface."""

    @abstractmethod
    async def index(self, data: list[VectorSearchRequest]) -> None:
        """Embed a list of documents.

        The embedding service accepts a massive list of id,strings to embed. Behind the
        scenes it batches up requests and parallelizes them for performance according to
        the specifics of the embedding service.

        The id reference is required because the parallelization may return results out
        of order.
        """

    @abstractmethod
    async def retrieve(self, query: str, top_k: int = 10) -> list[VectorSearchResponse]:
        """Query the embedding model."""
