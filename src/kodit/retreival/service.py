"""Retrieval service."""

from pathlib import Path

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

    def __init__(self, repository: RetrievalRepository, data_dir: Path) -> None:
        """Initialize the retrieval service."""
        self.repository = repository
        self.log = structlog.get_logger(__name__)
        self.bm25 = BM25Service(data_dir)

    async def _load_bm25_index(self) -> None:
        """Load the BM25 index."""

    async def retrieve(self, request: RetrievalRequest) -> list[RetrievalResult]:
        """Retrieve relevant data."""
        snippet_ids = await self.repository.list_snippet_ids()

        # Gather results for each keyword
        result_ids: list[tuple[int, float]] = []
        for keyword in request.keywords:
            results = self.bm25.retrieve(snippet_ids, keyword, request.top_k)
            result_ids.extend(results)

        if len(result_ids) == 0:
            return []

        # Sort results by score
        result_ids.sort(key=lambda x: x[1], reverse=True)

        self.log.debug(
            "Retrieval results",
            total_results=len(result_ids),
            max_score=result_ids[0][1],
            min_score=result_ids[-1][1],
            median_score=result_ids[len(result_ids) // 2][1],
        )

        # Don't return zero score results
        result_ids = [x for x in result_ids if x[1] > 0]

        # Build final list of doc ids up to top_k
        final_doc_ids = [x[0] for x in result_ids[: request.top_k]]

        # Get snippets from database
        return await self.repository.list_snippets_by_ids(final_doc_ids)
