"""Retrieval service."""

from pathlib import Path

import pydantic
import structlog

from kodit.bm25.bm25 import BM25Service
from kodit.embedding.embedding import EmbeddingService
from kodit.embedding.models import EmbeddingType
from kodit.retreival.repository import RetrievalRepository, RetrievalResult


class RetrievalRequest(pydantic.BaseModel):
    """Request for a retrieval."""

    code_query: str | None = None
    keywords: list[str] | None = None
    top_k: int = 10


class Snippet(pydantic.BaseModel):
    """Snippet model."""

    content: str
    file_path: str


class RetrievalService:
    """Service for retrieving relevant data."""

    def __init__(
        self,
        repository: RetrievalRepository,
        data_dir: Path,
        embedding_model_name: str,
    ) -> None:
        """Initialize the retrieval service."""
        self.repository = repository
        self.log = structlog.get_logger(__name__)
        self.bm25 = BM25Service(data_dir)
        self.code_embedding_service = EmbeddingService(model_name=embedding_model_name)

    async def retrieve(self, request: RetrievalRequest) -> list[RetrievalResult]:
        """Retrieve relevant data."""
        fusion_list = []
        if request.keywords:
            snippet_ids = await self.repository.list_snippet_ids()

            # Gather results for each keyword
            result_ids: list[tuple[int, float]] = []
            for keyword in request.keywords:
                results = self.bm25.retrieve(snippet_ids, keyword, request.top_k)
                result_ids.extend(results)

            # Sort results by score
            result_ids.sort(key=lambda x: x[1], reverse=True)

            self.log.debug("Retrieval results (BM25)", results=result_ids)

            bm25_results = [x[0] for x in result_ids]
            fusion_list.append(bm25_results)

        # Compute embedding for semantic query
        semantic_results = []
        if request.code_query:
            query_embedding = next(
                self.code_embedding_service.query([request.code_query])
            )

            query_results = await self.repository.list_semantic_results(
                EmbeddingType.CODE, query_embedding, top_k=request.top_k
            )

            # Sort results by score
            query_results.sort(key=lambda x: x[1], reverse=True)

            # Extract the snippet ids from the query results
            semantic_results = [x[0] for x in query_results]
            fusion_list.append(semantic_results)

        if len(fusion_list) == 0:
            return []

        # Combine all results together with RFF if required
        final_results = reciprocal_rank_fusion(fusion_list, k=60)

        # Extract ids from final results
        final_ids = [x[0] for x in final_results]

        # Get snippets from database (up to top_k)
        return await self.repository.list_snippets_by_ids(final_ids[: request.top_k])


def reciprocal_rank_fusion(
    rankings: list[list[int]], k: float = 60
) -> list[tuple[int, float]]:
    """RRF prioritises results that are present in all results.

    Args:
        rankings: List of rankers, each containing a list of document ids. Top of the
        list is considered to be the best result.
        k: Parameter for RRF.

    Returns:
        Dictionary of ids and their scores.

    """
    scores = {}
    for ranker in rankings:
        for rank in ranker:
            scores[rank] = float(0)

    for ranker in rankings:
        for i, rank in enumerate(ranker):
            scores[rank] += 1.0 / (k + i)

    # Create a list of tuples of ids and their scores
    results = [(rank, scores[rank]) for rank in scores]

    # Sort results by score
    results.sort(key=lambda x: x[1], reverse=True)

    return results
