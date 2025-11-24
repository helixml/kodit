"""Service for searching the indexes."""

from dataclasses import dataclass
from typing import TYPE_CHECKING

import structlog

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.entities.git import SnippetV2
from kodit.domain.protocols import FusionService
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.value_objects import (
    Enrichment,
    FusionRequest,
    LanguageMapping,
    MultiSearchRequest,
    SearchRequest,
    SearchResult,
)
from kodit.log import log_event

if TYPE_CHECKING:
    from kodit.application.services.enrichment_query_service import (
        EnrichmentQueryService,
    )


@dataclass
class MultiSearchResult:
    """Enhanced search result with comprehensive snippet metadata."""

    snippet: SnippetV2
    original_scores: list[float]
    enrichment_type: str
    enrichment_subtype: str | None

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

    @classmethod
    def to_markdown(cls, results: list["MultiSearchResult"]) -> str:
        """Convert multiple MultiSearchResult objects to Markdown format."""
        if not results:
            return "# Search Results (0 matches)\n\nNo results found."

        lines = [f"# Search Results ({len(results)} matches)\n"]

        for i, result in enumerate(results):
            # Determine filename from enrichment type/subtype
            filename = cls._filename(result)

            # Add separator between results (except before first)
            if i > 0:
                lines.append("\n---\n")

            # Add heading with filename
            lines.append(f"## {filename}\n")

            # Add metadata
            lines.append("**Metadata:**")
            lines.append(f"- Type: {result.enrichment_type}")
            if result.enrichment_subtype:
                lines.append(f"- Subtype: {result.enrichment_subtype}")

            # Determine language from extension
            language = cls._language(result.snippet.extension)
            if language:
                lines.append(f"- Language: {language}")

            # Add scores
            if result.original_scores:
                scores_str = ", ".join(f"{s:.4f}" for s in result.original_scores)
                lines.append(f"- Score: {scores_str}")

            # Add code block
            lines.append(f"\n```{language}")
            lines.append(result.snippet.content)
            lines.append("```")

            # Add enrichments if they exist
            if result.snippet.enrichments:
                lines.append("\n**Enrichments:**\n")
                for enrichment in result.snippet.enrichments:
                    lines.append(f"- **{enrichment.type}:**")
                    lines.append("  ```")
                    lines.append(f"  {enrichment.content}")
                    lines.append("  ```")

        return "\n".join(lines)

    @staticmethod
    def _filename(result: "MultiSearchResult") -> str:
        """Generate filename from enrichment type and subtype."""
        if result.enrichment_subtype:
            return f"{result.enrichment_type}/{result.enrichment_subtype}"
        return result.enrichment_type

    @staticmethod
    def _language(extension: str) -> str:
        """Get language identifier from file extension."""
        if not extension:
            return ""

        try:
            return LanguageMapping.get_language_for_extension(extension)
        except ValueError:
            # If extension not recognized, return it as-is
            return extension.removeprefix(".")


class CodeSearchApplicationService:
    """Service for searching the indexes."""

    def __init__(  # noqa: PLR0913
        self,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        progress_tracker: ProgressTracker,
        fusion_service: FusionService,
        enrichment_query_service: "EnrichmentQueryService",
    ) -> None:
        """Initialize the code search application service."""
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.progress_tracker = progress_tracker
        self.fusion_service = fusion_service
        self.enrichment_query_service = enrichment_query_service
        self.log = structlog.get_logger(__name__)

    async def search(self, request: MultiSearchRequest) -> list[MultiSearchResult]:  # noqa: C901, PLR0912
        """Search for relevant snippets across all indexes."""
        log_event("kodit.index.search")

        # Apply commit SHA filter if provided
        filtered_snippet_ids: list[str] | None = None
        if request.filters and request.filters.commit_sha:
            # Get all enrichments associated with these commits
            all_associations = []
            for commit_sha in request.filters.commit_sha:
                associations = (
                    await self.enrichment_query_service.associations_for_commit(
                        commit_sha
                    )
                )
                all_associations.extend(associations)

            # Extract unique enrichment IDs as strings for filtering
            filtered_snippet_ids = list(
                {str(assoc.enrichment_id) for assoc in all_associations}
            )
            if not filtered_snippet_ids:
                # No enrichments for these commits, return empty results
                return []

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
            # These contain a pointer to the enrichment ID that represents the summary
            summary_results = await self.text_search_service.search(
                SearchRequest(
                    query=request.text_query,
                    top_k=request.top_k,
                    snippet_ids=filtered_snippet_ids,
                )
            )

            summary_to_snippet_map = (
                await self.enrichment_query_service.summary_to_snippet_map(
                    summary_ids=[int(x.snippet_id) for x in summary_results]
                )
            )

            # Build fusion list in the correct order
            fusion_items = [
                FusionRequest(
                    id=str(summary_to_snippet_map[int(result.snippet_id)]),
                    score=result.score,
                )
                for result in summary_results
                if int(result.snippet_id) in summary_to_snippet_map
            ]
            fusion_list.append(fusion_items)

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
        final_enrichments = await self.enrichment_query_service.get_enrichments_by_ids(
            enrichment_ids
        )

        # Apply all filters if provided
        if request.filters:
            # Filter by enrichment type
            if request.filters.enrichment_types:
                final_enrichments = [
                    e
                    for e in final_enrichments
                    if e.type in request.filters.enrichment_types
                ]
            # Filter by enrichment subtype
            if request.filters.enrichment_subtypes:
                final_enrichments = [
                    e
                    for e in final_enrichments
                    if e.subtype in request.filters.enrichment_subtypes
                ]
            # Filter by created_after date
            if request.filters.created_after:
                final_enrichments = [
                    e
                    for e in final_enrichments
                    if e.created_at and e.created_at >= request.filters.created_after
                ]
            # Filter by created_before date
            if request.filters.created_before:
                final_enrichments = [
                    e
                    for e in final_enrichments
                    if e.created_at and e.created_at <= request.filters.created_before
                ]

        # Get enrichments pointing to these enrichments
        extra_enrichments = (
            await self.enrichment_query_service.get_enrichments_pointing_to_enrichments(
                [e.id for e in final_enrichments if e.id]
            )
        )

        self.log.info(
            "final enrichments",
            len_final_enrichments=len(final_enrichments),
        )

        # Convert enrichments to SnippetV2 domain objects
        # Map enrichment ID to snippet and type info for correct ordering
        enrichment_id_to_snippet: dict[int | None, SnippetV2] = {}
        enrichment_id_to_type: dict[int | None, tuple[str, str | None]] = {}
        for enrichment in final_enrichments:
            # Get extra enrichments for this enrichment (only if ID is not None)
            enrichment_extras = (
                extra_enrichments[enrichment.id] if enrichment.id is not None else []
            )
            enrichment_id_to_snippet[enrichment.id] = SnippetV2(
                sha=str(enrichment.id),  # The snippet SHA
                content=enrichment.content,  # The code content
                extension="",  # Not available in enrichment
                derives_from=[],  # Not available in enrichment
                created_at=enrichment.created_at,
                updated_at=enrichment.updated_at,
                enrichments=[
                    Enrichment(
                        type=enrichment.subtype or enrichment.type,
                        content=enrichment.content,
                    )
                    for enrichment in enrichment_extras
                ],
            )
            enrichment_id_to_type[enrichment.id] = (
                enrichment.type,
                enrichment.subtype,
            )

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
                enrichment_type=enrichment_id_to_type[int(snippet.sha)][0],
                enrichment_subtype=enrichment_id_to_type[int(snippet.sha)][1],
            )
            for snippet in snippets
        ]
