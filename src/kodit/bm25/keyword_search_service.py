"""Keyword search service."""

from abc import ABC, abstractmethod
from typing import NamedTuple


class BM25Document(NamedTuple):
    """BM25 document."""

    snippet_id: int
    text: str


class BM25Result(NamedTuple):
    """BM25 result."""

    snippet_id: int
    score: float


class KeywordSearchProvider(ABC):
    """Interface for keyword search providers."""

    @abstractmethod
    async def index(self, corpus: list[BM25Document]) -> None:
        """Index a new corpus."""

    @abstractmethod
    async def retrieve(self, query: str, top_k: int = 2) -> list[BM25Result]:
        """Retrieve from the index."""

    @abstractmethod
    async def delete(self, snippet_ids: list[int]) -> None:
        """Delete documents from the index."""
