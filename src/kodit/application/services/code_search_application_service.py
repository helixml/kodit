"""Service for searching the indexes."""

from dataclasses import replace
from datetime import UTC, datetime

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.index_query_service import IndexQueryService
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
        index_query_service: IndexQueryService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        progress_tracker: ProgressTracker,
    ) -> None:
        """Initialize the code search application service."""
        self.index_query_service = index_query_service
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.progress_tracker = progress_tracker
        self.log = structlog.get_logger(__name__)

    async def search(self, request: MultiSearchRequest) -> list[MultiSearchResult]:
        """Search for relevant snippets across all indexes."""
        log_event("kodit.index.search")

        # Apply filters if provided
        filtered_snippet_ids: list[int] | None = None
        if request.filters:
            # Use domain service for filtering (use large top_k for pre-filtering)
            prefilter_request = replace(request, top_k=10000)
            snippet_results = await self.index_query_service.search_snippets(
                prefilter_request
            )
            filtered_snippet_ids = [
                snippet.snippet.id for snippet in snippet_results if snippet.snippet.id
            ]

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
        final_results = await self.index_query_service.perform_fusion(
            rankings=fusion_list,
            k=60,  # This is a parameter in the RRF algorithm, not top_k
        )

        # Keep only top_k results
        final_results = final_results[: request.top_k]

        # Get snippet details
        search_results = await self.index_query_service.get_snippets_by_ids(
            [x.id for x in final_results]
        )

        # Create a mapping from snippet ID to search result to handle cases where
        # some snippet IDs don't exist (e.g., with vectorchord inconsistencies)
        snippet_map = {
            result.snippet.id: result
            for result in search_results
            if result.snippet.id is not None
        }

        # Filter final_results to only include IDs that we actually found snippets for
        valid_final_results = [fr for fr in final_results if fr.id in snippet_map]

        return [
            MultiSearchResult(
                id=snippet_map[fr.id].snippet.id or 0,
                content=snippet_map[fr.id].snippet.original_text(),
                original_scores=fr.original_scores,
                # Enhanced fields
                source_uri=str(snippet_map[fr.id].source.working_copy.remote_uri),
                relative_path=str(
                    snippet_map[fr.id]
                    .file.as_path()
                    .relative_to(snippet_map[fr.id].source.working_copy.cloned_path)
                ),
                language=MultiSearchResult.detect_language_from_extension(
                    snippet_map[fr.id].file.extension()
                ),
                authors=[author.name for author in snippet_map[fr.id].authors],
                created_at=snippet_map[fr.id].snippet.created_at or datetime.now(UTC),
                # Summary from snippet entity
                summary=snippet_map[fr.id].snippet.summary_text(),
            )
            for fr in valid_final_results
        ]
