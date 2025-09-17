"""Service for searching the indexes."""

from datetime import UTC, datetime

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.protocols import FusionService, SnippetRepositoryV2
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.value_objects import (
    FusionRequest,
    MultiSearchRequest,
    MultiSearchResult,
    SearchRequest,
    SearchResult,
)
from kodit.log import log_event


class CodeSearchApplicationService:
    """Service for searching the indexes."""

    def __init__(
        self,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        progress_tracker: ProgressTracker,
        snippet_repository: SnippetRepositoryV2,
        fusion_service: FusionService,
    ) -> None:
        """Initialize the code search application service."""
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.progress_tracker = progress_tracker
        self.snippet_repository = snippet_repository
        self.fusion_service = fusion_service
        self.log = structlog.get_logger(__name__)

    async def search(self, request: MultiSearchRequest) -> list[MultiSearchResult]:
        """Search for relevant snippets across all indexes."""
        log_event("kodit.index.search")

        # Apply filters if provided
        filtered_snippet_ids: list[int] | None = None
        # if request.filters:
        #     # Use domain service for filtering (use large top_k for pre-filtering)
        #     prefilter_request = replace(request, top_k=10000)
        #     snippet_results = await self.snippet_repository.search(prefilter_request)
        #     filtered_snippet_ids = [
        #         snippet.snippet.id for snippet in snippet_results if snippet.snippet.id
        #     ]

        # Gather results from different search modes
        fusion_list: list[list[FusionRequest]] = []

        # Keyword search
        if request.keywords:
            result_ids: list[SearchResult] = []
            for keyword in request.keywords:
                results = await self.bm25_service.search(
                    SearchRequest(
                        query=keyword,
                        top_k=request.top_k,
                        snippet_ids=filtered_snippet_ids,
                    )
                )
                result_ids.extend(results)

            fusion_list.append(
                [FusionRequest(id=x.snippet_id, score=x.score) for x in result_ids]
            )

        # Semantic code search
        if request.code_query:
            query_results = await self.code_search_service.search(
                SearchRequest(
                    query=request.code_query,
                    top_k=request.top_k,
                    snippet_ids=filtered_snippet_ids,
                )
            )
            fusion_list.append(
                [FusionRequest(id=x.snippet_id, score=x.score) for x in query_results]
            )

        # Semantic text search
        if request.text_query:
            query_results = await self.text_search_service.search(
                SearchRequest(
                    query=request.text_query,
                    top_k=request.top_k,
                    snippet_ids=filtered_snippet_ids,
                )
            )
            fusion_list.append(
                [FusionRequest(id=x.snippet_id, score=x.score) for x in query_results]
            )

        if len(fusion_list) == 0:
            return []

        # Fusion ranking
        final_results = self.fusion_service.reciprocal_rank_fusion(
            rankings=fusion_list,
            k=60,  # This is a parameter in the RRF algorithm, not top_k
        )

        # Keep only top_k results
        final_results = final_results[: request.top_k]

        # Get snippet details
        ids = [x.id for x in final_results]
        search_results = await self.snippet_repository.get_by_ids(ids)
        search_results.sort(key=lambda x: ids.index(x.id))
        return [
            MultiSearchResult(
                id=snippet.id,
                content=snippet.content,
                original_scores=[x.score for x in final_results if x.id == snippet.id],
                # Enhanced fields
                source_uri="N/A",  # TODO(Phil): What to do about these fields...
                relative_path="N/A",
                language=MultiSearchResult.detect_language_from_extension(
                    snippet.extension
                ),
                authors=[],
                created_at=snippet.created_at or datetime.now(UTC),
                # Summary from snippet entity
                summary="N/A",
            )
            for snippet in search_results
        ]
