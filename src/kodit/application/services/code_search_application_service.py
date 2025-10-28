"""Service for searching the indexes."""

from dataclasses import dataclass

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities.git import SnippetV2
from kodit.domain.protocols import (
    EnrichmentAssociationRepository,
    EnrichmentV2Repository,
    FusionService,
)
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.value_objects import (
    FusionRequest,
    MultiSearchRequest,
    SearchRequest,
    SearchResult,
)
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.query import (
    FilterOperator,
    QueryBuilder,
)
from kodit.log import log_event


@dataclass
class MultiSearchResult:
    """Enhanced search result with comprehensive snippet metadata."""

    snippet: SnippetV2
    original_scores: list[float]

    def to_json(self) -> str:
        """Return LLM-optimized JSON representation following the compact schema."""
        return self.snippet.model_dump_json()

    @classmethod
    def to_jsonlines(cls, results: list["MultiSearchResult"]) -> str:
        """Convert multiple MultiSearchResult objects to JSON Lines format.

        Args:
            results: List of MultiSearchResult objects
            include_summary: Whether to include summary fields

        Returns:
            JSON Lines string (one JSON object per line)

        """
        return "\n".join(result.to_json() for result in results)


class CodeSearchApplicationService:
    """Service for searching the indexes."""

    def __init__(  # noqa: PLR0913
        self,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        progress_tracker: ProgressTracker,
        fusion_service: FusionService,
        enrichment_v2_repository: "EnrichmentV2Repository",
        enrichment_association_repository: EnrichmentAssociationRepository,
    ) -> None:
        """Initialize the code search application service."""
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.progress_tracker = progress_tracker
        self.fusion_service = fusion_service
        self.enrichment_v2_repository = enrichment_v2_repository
        self.enrichment_association_repository = enrichment_association_repository
        self.log = structlog.get_logger(__name__)

    async def search(self, request: MultiSearchRequest) -> list[MultiSearchResult]:
        """Search for relevant snippets across all indexes."""
        log_event("kodit.index.search")

        # Apply filters if provided
        filtered_snippet_ids: list[str] | None = None
        # TODO(Phil): Re-implement filtering on search results

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
            summary_enrichment_associations = await self.enrichment_association_repository.get_snippet_associations_for_summary_enrichments(
                [int(x.snippet_id) for x in query_results]
            )
            fusion_list.append(
                [
                    FusionRequest(
                        id=str(summary_enrichment_association.enrichment_id),
                        score=query_result.score,
                    )
                    for query_result, summary_enrichment_association in zip(
                        query_results, summary_enrichment_associations, strict=False
                    )
                ]
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

        # Get enrichment details
        enrichment_ids = [int(x.id) for x in final_results]

        self.log.info(
            "found enrichments",
            len_enrichments=len(enrichment_ids),
        )
        final_enrichments = await self.enrichment_v2_repository.find(
            QueryBuilder().filter(
                db_entities.EnrichmentV2.id.key,
                FilterOperator.IN,
                enrichment_ids,
            )
        )

        self.log.info(
            "final enrichments",
            len_final_enrichments=len(final_enrichments),
        )

        # Convert enrichments to SnippetV2 domain objects
        # Map enrichment ID to snippet for correct ordering
        enrichment_id_to_snippet: dict[int, SnippetV2] = {}
        for enrichment in final_enrichments:
            if not enrichment.id:
                raise ValueError(f"Enrichment {enrichment} has no ID")
            # Create a SnippetV2 from the enrichment
            snippet = SnippetV2(
                sha=str(enrichment.id),  # The snippet SHA
                content=enrichment.content,  # The code content
                extension="",  # Not available in enrichment
                derives_from=[],  # Not available in enrichment
                created_at=enrichment.created_at,
                updated_at=enrichment.updated_at,
                enrichments=[],  # Could fetch summaries if needed
            )
            enrichment_id_to_snippet[enrichment.id] = snippet

        # Sort by the original fusion ranking order
        snippets = [
            enrichment_id_to_snippet[eid]
            for eid in enrichment_ids
            if eid in enrichment_id_to_snippet
        ]

        return [
            MultiSearchResult(
                snippet=snippet,
                original_scores=[
                    x.score
                    for x in final_results
                    if int(x.id) in enrichment_id_to_snippet
                    and enrichment_id_to_snippet[int(x.id)].sha == snippet.sha
                ],
            )
            for snippet in snippets
        ]
