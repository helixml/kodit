"""Unified application service for code indexing operations."""

from dataclasses import replace

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Snippet
from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.errors import EmptySourceError
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.indexing_service import IndexingDomainService
from kodit.domain.services.snippet_service import SnippetDomainService
from kodit.domain.services.source_service import SourceService
from kodit.domain.value_objects import (
    Document,
    EnrichmentIndexRequest,
    EnrichmentRequest,
    FusionRequest,
    IndexCreateRequest,
    IndexRequest,
    IndexView,
    MultiSearchRequest,
    MultiSearchResult,
    SearchRequest,
    SearchResult,
)
from kodit.log import log_event
from kodit.reporting import Reporter


class CodeIndexingApplicationService:
    """Unified application service for all code indexing operations."""

    def __init__(  # noqa: PLR0913
        self,
        indexing_domain_service: IndexingDomainService,
        snippet_domain_service: SnippetDomainService,
        source_service: SourceService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        enrichment_service: EnrichmentDomainService,
        session: AsyncSession,
    ) -> None:
        """Initialize the code indexing application service."""
        self.indexing_domain_service = indexing_domain_service
        self.snippet_domain_service = snippet_domain_service
        self.source_service = source_service
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.enrichment_service = enrichment_service
        self.session = session
        self.log = structlog.get_logger(__name__)

    async def create_index(self, source_id: int) -> IndexView:
        """Create a new index for a source."""
        log_event("kodit.index.create")

        # Validate source exists
        source = await self.source_service.get(source_id)

        # Create index
        request = IndexCreateRequest(source_id=source.id)
        index_view = await self.indexing_domain_service.create_index(request)

        # Single transaction commit
        await self.session.commit()

        return index_view

    async def list_indexes(self) -> list[IndexView]:
        """List all available indexes with their details."""
        indexes = await self.indexing_domain_service.list_indexes()

        # Telemetry
        log_event(
            "kodit.index.list",
            {
                "num_indexes": len(indexes),
                "num_snippets": sum([index.num_snippets for index in indexes]),
            },
        )

        return indexes

    async def run_index(
        self, index_id: int, progress_callback: ProgressCallback | None = None
    ) -> None:
        """Run the complete indexing process for a specific index."""
        log_event("kodit.index.run")

        # Validate index
        index = await self.indexing_domain_service.get_index(index_id)
        if not index:
            msg = f"Index not found: {index_id}"
            raise ValueError(msg)

        # Delete old snippets to make way for reindexing
        # In the future we will only reindex snippets that have changed
        await self.snippet_domain_service.delete_snippets_for_index(index.id)

        # Extract and create snippets (domain service handles progress)
        self.log.info("Creating snippets for files", index_id=index.id)
        snippets = await self.snippet_domain_service.extract_and_create_snippets(
            index_id=index.id,
            strategy=SnippetExtractionStrategy.METHOD_BASED,
            progress_callback=progress_callback,
        )

        # Check if any snippets were extracted
        if not snippets:
            msg = f"No indexable snippets found for index {index.id}"
            raise EmptySourceError(msg)

        # Commit snippets to ensure they have IDs for indexing
        await self.session.commit()

        # Create BM25 index
        self.log.info("Creating keyword index")
        await self._create_bm25_index(snippets, progress_callback)

        # Create code embeddings
        self.log.info("Creating semantic code index")
        await self._create_code_embeddings(snippets, progress_callback)

        # Enrich snippets
        self.log.info("Enriching snippets", num_snippets=len(snippets))
        await self._enrich_snippets(snippets, progress_callback)

        # Get refreshed snippets after enrichment
        snippets = await self.snippet_domain_service.get_snippets_for_index(index.id)

        # Create text embeddings (on enriched content)
        self.log.info("Creating semantic text index")
        await self._create_text_embeddings(snippets, progress_callback)

        # Update index timestamp
        await self.indexing_domain_service.update_index_timestamp(index.id)

        # Single transaction commit for the entire operation
        await self.session.commit()

    async def search(self, request: MultiSearchRequest) -> list[MultiSearchResult]:
        """Search for relevant snippets across all indexes."""
        log_event("kodit.index.search")

        # Apply filters if provided
        filtered_snippet_ids: list[int] | None = None
        if request.filters:
            # Use domain service for filtering
            prefilter_request = replace(request, top_k=None)
            snippet_results = await self.snippet_domain_service.search_snippets(
                prefilter_request
            )
            filtered_snippet_ids = [snippet.id for snippet in snippet_results]

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
        final_results = self.indexing_domain_service.perform_fusion(
            rankings=fusion_list,
            k=60,  # This is a parameter in the RRF algorithm, not top_k
        )

        # Keep only top_k results
        final_results = final_results[: request.top_k]

        # Get snippet details
        search_results = await self.indexing_domain_service.get_snippets_by_ids(
            [x.id for x in final_results]
        )

        return [
            MultiSearchResult(
                id=result.snippet.id,
                uri=result.file.uri,
                content=result.snippet.content,
                original_scores=fr.original_scores,
            )
            for result, fr in zip(search_results, final_results, strict=True)
        ]

    async def list_snippets(
        self, file_path: str | None = None, source_uri: str | None = None
    ) -> list[MultiSearchResult]:
        """List snippets with optional filtering."""
        log_event("kodit.index.list_snippets")
        return await self.snippet_domain_service.list_snippets(file_path, source_uri)

    async def _create_bm25_index(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        reporter = Reporter(self.log, progress_callback)
        await reporter.start("bm25_index", len(snippets), "Creating keyword index...")

        await self.bm25_service.index_documents(
            IndexRequest(
                documents=[
                    Document(snippet_id=snippet.id, text=snippet.content)
                    for snippet in snippets
                ]
            )
        )

        await reporter.done("bm25_index", "Keyword index created")

    async def _create_code_embeddings(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "code_embeddings", len(snippets), "Creating code embeddings..."
        )

        processed = 0
        async for result in self.code_search_service.index_documents(
            IndexRequest(
                documents=[
                    Document(snippet_id=snippet.id, text=snippet.content)
                    for snippet in snippets
                ]
            )
        ):
            processed += len(result)
            await reporter.step(
                "code_embeddings",
                processed,
                len(snippets),
                "Creating code embeddings...",
            )

        await reporter.done("code_embeddings")

    async def _enrich_snippets(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        reporter = Reporter(self.log, progress_callback)
        await reporter.start("enrichment", len(snippets), "Enriching snippets...")

        enrichment_request = EnrichmentIndexRequest(
            requests=[
                EnrichmentRequest(snippet_id=snippet.id, text=snippet.content)
                for snippet in snippets
            ]
        )

        processed = 0
        async for result in self.enrichment_service.enrich_documents(
            enrichment_request
        ):
            # Update snippet content through domain service
            enriched_content = (
                result.text
                + "\n\n```\n"
                + next(s.content for s in snippets if s.id == result.snippet_id)
                + "\n```"
            )

            await self.snippet_domain_service.update_snippet_content(
                result.snippet_id, enriched_content
            )

            processed += 1
            await reporter.step(
                "enrichment", processed, len(snippets), "Enriching snippets..."
            )

        await reporter.done("enrichment")

    async def _create_text_embeddings(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "text_embeddings", len(snippets), "Creating text embeddings..."
        )

        processed = 0
        async for result in self.text_search_service.index_documents(
            IndexRequest(
                documents=[
                    Document(snippet_id=snippet.id, text=snippet.content)
                    for snippet in snippets
                ]
            )
        ):
            processed += len(result)
            await reporter.step(
                "text_embeddings",
                processed,
                len(snippets),
                "Creating text embeddings...",
            )

        await reporter.done("text_embeddings")
