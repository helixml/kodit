"""Retrieval service."""

import pydantic
import structlog

from kodit.bm25.bm25 import BM25Service
from kodit.retreival.repository import RetrievalRepository, RetrievalResult


class RetrievalRequest(pydantic.BaseModel):
    """Request for a retrieval."""

    keywords: list[str]
    top_k: int = 10


class Snippet(pydantic.BaseModel):
    """Snippet model."""

    content: str
    file_path: str


class RetrievalService:
    """Service for retrieving relevant data."""

    def __init__(self, repository: RetrievalRepository) -> None:
        """Initialize the retrieval service."""
        self.repository = repository
        self.log = structlog.get_logger(__name__)
        self.bm25 = BM25Service()

    async def _load_bm25_index(self) -> None:
        """Load the BM25 index."""

    async def retrieve(self, request: RetrievalRequest) -> list[RetrievalResult]:
        """Retrieve relevant data."""
        snippet_ids = await self.repository.list_snippet_ids()
        results = self.bm25.retrieve(snippet_ids, request.keywords[0], request.top_k)
        # Get results from database
        return await self.repository.list_snippets_by_ids(results)
