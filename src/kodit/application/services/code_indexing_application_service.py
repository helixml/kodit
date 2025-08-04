"""Unified application service for code indexing operations."""

from dataclasses import replace
from datetime import UTC, datetime

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Index, Snippet
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.protocols import IndexRepository
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.services.index_service import IndexDomainService
from kodit.domain.value_objects import (
    Document,
    FusionRequest,
    IndexRequest,
    MultiSearchRequest,
    MultiSearchResult,
    SearchRequest,
    SearchResult,
    SnippetSearchFilters,
)
from kodit.log import log_event
from kodit.reporting import Reporter


class CodeIndexingApplicationService:
    """Unified application service for all code indexing operations."""

    def __init__(  # noqa: PLR0913
        self,
        indexing_domain_service: IndexDomainService,
        index_repository: IndexRepository,
        index_query_service: IndexQueryService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        enrichment_service: EnrichmentDomainService,
        session: AsyncSession,
    ) -> None:
        """Initialize the code indexing application service."""
        self.index_domain_service = indexing_domain_service
        self.index_repository = index_repository
        self.index_query_service = index_query_service
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.enrichment_service = enrichment_service
        self.session = session
        self.log = structlog.get_logger(__name__)

    async def does_index_exist(self, uri: str) -> bool:
        """Check if an index exists for a source."""
        # Check if index already exists
        sanitized_uri, _ = self.index_domain_service.sanitize_uri(uri)
        existing_index = await self.index_repository.get_by_uri(sanitized_uri)
        return existing_index is not None

    async def create_index_from_uri(
        self, uri: str, progress_callback: ProgressCallback | None = None
    ) -> Index:
        """Create a new index for a source."""
        log_event("kodit.index.create")

        # Check if index already exists
        sanitized_uri, _ = self.index_domain_service.sanitize_uri(uri)
        existing_index = await self.index_repository.get_by_uri(sanitized_uri)
        if existing_index:
            self.log.debug(
                "Index already exists",
                uri=str(sanitized_uri),
                index_id=existing_index.id,
            )
            return existing_index

        # Only prepare working copy if we need to create a new index
        working_copy = await self.index_domain_service.prepare_index(
            uri, progress_callback
        )

        # Create new index
        index = await self.index_repository.create(sanitized_uri, working_copy)
        await self.session.commit()
        return index

    async def run_index(
        self, index: Index, progress_callback: ProgressCallback | None = None
    ) -> None:
        """Run the complete indexing process for a specific index."""
        log_event("kodit.index.run")

        if not index or not index.id:
            msg = f"Index has no ID: {index}"
            raise ValueError(msg)

        # Refresh working copy
        index.source.working_copy = (
            await self.index_domain_service.refresh_working_copy(
                index.source.working_copy
            )
        )
        if len(index.source.working_copy.changed_files()) == 0:
            self.log.info("No new changes to index", index_id=index.id)
            return

        # Delete the old snippets from the files that have changed
        await self.index_repository.delete_snippets_by_file_ids(
            [file.id for file in index.source.working_copy.changed_files() if file.id]
        )

        # Extract and create snippets (domain service handles progress)
        self.log.info("Creating snippets for files", index_id=index.id)
        index = await self.index_domain_service.extract_snippets_from_index(
            index=index, progress_callback=progress_callback
        )

        await self.index_repository.update(index)
        await self.session.flush()

        # Refresh index to get snippets with IDs, required as a ref for subsequent steps
        flushed_index = await self.index_repository.get(index.id)
        if not flushed_index:
            msg = f"Index {index.id} not found after snippet extraction"
            raise ValueError(msg)
        index = flushed_index
        if len(index.snippets) == 0:
            self.log.info("No snippets to index after extraction", index_id=index.id)
            return

        # Create BM25 index
        self.log.info("Creating keyword index")
        await self._create_bm25_index(index.snippets, progress_callback)

        # Create code embeddings
        self.log.info("Creating semantic code index")
        await self._create_code_embeddings(index.snippets, progress_callback)

        # Enrich snippets
        self.log.info("Enriching snippets", num_snippets=len(index.snippets))
        enriched_snippets = await self.index_domain_service.enrich_snippets_in_index(
            snippets=index.snippets, progress_callback=progress_callback
        )
        # Update snippets in repository
        await self.index_repository.update_snippets(index.id, enriched_snippets)

        # Create text embeddings (on enriched content)
        self.log.info("Creating semantic text index")
        await self._create_text_embeddings(enriched_snippets, progress_callback)

        # Update index timestamp
        await self.index_repository.update_index_timestamp(index.id)

        # Now that all file dependencies have been captured, enact the file processing
        # statuses
        index.source.working_copy.clear_file_processing_statuses()
        await self.index_repository.update(index)

        # Single transaction commit for the entire operation
        await self.session.commit()

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

    async def list_snippets(
        self, file_path: str | None = None, source_uri: str | None = None
    ) -> list[MultiSearchResult]:
        """List snippets with optional filtering."""
        log_event("kodit.index.list_snippets")
        snippet_results = await self.index_query_service.search_snippets(
            request=MultiSearchRequest(
                filters=SnippetSearchFilters(
                    file_path=file_path,
                    source_repo=source_uri,
                )
            ),
        )
        return [
            MultiSearchResult(
                id=result.snippet.id or 0,
                content=result.snippet.original_text(),
                original_scores=[0.0],
                # Enhanced fields
                source_uri=str(result.source.working_copy.remote_uri),
                relative_path=str(
                    result.file.as_path().relative_to(
                        result.source.working_copy.cloned_path
                    )
                ),
                language=MultiSearchResult.detect_language_from_extension(
                    result.file.extension()
                ),
                authors=[author.name for author in result.authors],
                created_at=result.snippet.created_at or datetime.now(UTC),
                # Summary from snippet entity
                summary=result.snippet.summary_text(),
            )
            for result in snippet_results
        ]

    # FUTURE: BM25 index enriched content too
    async def _create_bm25_index(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        reporter = Reporter(self.log, progress_callback)
        await reporter.start("bm25_index", len(snippets), "Creating keyword index...")

        for _snippet in snippets:
            pass

        await self.bm25_service.index_documents(
            IndexRequest(
                documents=[
                    Document(snippet_id=snippet.id, text=snippet.original_text())
                    for snippet in snippets
                    if snippet.id
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
                    Document(snippet_id=snippet.id, text=snippet.original_text())
                    for snippet in snippets
                    if snippet.id
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

    async def _create_text_embeddings(
        self, snippets: list[Snippet], progress_callback: ProgressCallback | None = None
    ) -> None:
        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "text_embeddings", len(snippets), "Creating text embeddings..."
        )

        # Only create text embeddings for snippets that have summary content
        documents_with_summaries = []
        for snippet in snippets:
            if snippet.id:
                try:
                    summary_text = snippet.summary_text()
                    if summary_text.strip():  # Only add if summary is not empty
                        documents_with_summaries.append(
                            Document(snippet_id=snippet.id, text=summary_text)
                        )
                except ValueError:
                    # Skip snippets without summary content
                    continue

        if not documents_with_summaries:
            await reporter.done("text_embeddings", "No summaries to index")
            return

        processed = 0
        async for result in self.text_search_service.index_documents(
            IndexRequest(documents=documents_with_summaries)
        ):
            processed += len(result)
            await reporter.step(
                "text_embeddings",
                processed,
                len(snippets),
                "Creating text embeddings...",
            )

        await reporter.done("text_embeddings")

    async def delete_index(self, index: Index) -> None:
        """Delete an index."""
        # Delete the index from the domain
        await self.index_domain_service.delete_index(index)

        # Delete index from the database
        await self.index_repository.delete(index)
        await self.session.commit()
