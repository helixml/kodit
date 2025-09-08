"""Unified application service for code indexing operations."""

from datetime import UTC, datetime

import structlog

from kodit.application.services.reporting import (
    ProgressTracker,
    TaskOperation,
)
from kodit.domain.entities import Index, Snippet
from kodit.domain.protocols import IndexRepository
from kodit.domain.services.bm25_service import BM25DomainService
from kodit.domain.services.embedding_service import EmbeddingDomainService
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.services.index_service import IndexDomainService
from kodit.domain.value_objects import (
    Document,
    IndexRequest,
    MultiSearchRequest,
    MultiSearchResult,
    SnippetSearchFilters,
    TrackableType,
)
from kodit.log import log_event


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
        operation: ProgressTracker,
    ) -> None:
        """Initialize the code indexing application service."""
        self.index_domain_service = indexing_domain_service
        self.index_repository = index_repository
        self.index_query_service = index_query_service
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.enrichment_service = enrichment_service
        self.operation = operation
        self.log = structlog.get_logger(__name__)

    async def does_index_exist(self, uri: str) -> bool:
        """Check if an index exists for a source."""
        # Check if index already exists
        sanitized_uri, _ = self.index_domain_service.sanitize_uri(uri)
        existing_index = await self.index_repository.get_by_uri(sanitized_uri)
        return existing_index is not None

    async def create_index_from_uri(self, uri: str) -> Index:
        """Create a new index for a source."""
        async with self.operation.create_child(TaskOperation.CREATE_INDEX) as operation:
            # Check if index already exists
            sanitized_uri, _ = self.index_domain_service.sanitize_uri(uri)
            self.log.info("Creating index from URI", uri=str(sanitized_uri))
            existing_index = await self.index_repository.get_by_uri(sanitized_uri)
            if existing_index:
                self.log.debug(
                    "Index already exists",
                    uri=str(sanitized_uri),
                    index_id=existing_index.id,
                )
                return existing_index

            # Only prepare working copy if we need to create a new index
            self.log.info("Preparing working copy", uri=str(sanitized_uri))
            working_copy = await self.index_domain_service.prepare_index(uri, operation)

            # Create new index
            self.log.info("Creating index", uri=str(sanitized_uri))
            return await self.index_repository.create(sanitized_uri, working_copy)

    async def run_index(self, index: Index) -> None:
        """Run the complete indexing process for a specific index."""
        # Create a new operation
        async with self.operation.create_child(
            TaskOperation.RUN_INDEX,
            trackable_type=TrackableType.INDEX,
            trackable_id=index.id,
        ) as operation:
            if not index or not index.id:
                msg = f"Index has no ID: {index}"
                raise ValueError(msg)

            # Refresh working copy
            async with operation.create_child(
                TaskOperation.REFRESH_WORKING_COPY
            ) as step:
                index.source.working_copy = (
                    await self.index_domain_service.refresh_working_copy(
                        index.source.working_copy, step
                    )
                )
                if len(index.source.working_copy.changed_files()) == 0:
                    self.log.info("No new changes to index", index_id=index.id)
                    await step.skip("No new changes to index")
                    return

            # Delete the old snippets from the files that have changed
            async with operation.create_child(
                TaskOperation.DELETE_OLD_SNIPPETS
            ) as step:
                await self.index_repository.delete_snippets_by_file_ids(
                    [
                        file.id
                        for file in index.source.working_copy.changed_files()
                        if file.id
                    ]
                )

            # Extract and create snippets (domain service handles progress)
            async with operation.create_child(TaskOperation.EXTRACT_SNIPPETS) as step:
                index = await self.index_domain_service.extract_snippets_from_index(
                    index=index, step=step
                )
                await self.index_repository.update(index)

                # Refresh index to get snippets with IDs, required for subsequent steps
                flushed_index = await self.index_repository.get(index.id)
                if not flushed_index:
                    msg = f"Index {index.id} not found after snippet extraction"
                    raise ValueError(msg)
                index = flushed_index
                if len(index.snippets) == 0:
                    self.log.info(
                        "No snippets to index after extraction", index_id=index.id
                    )
                    await step.skip("No snippets to index after extraction")
                    return

            # Create BM25 index
            self.log.info("Creating keyword index")
            async with operation.create_child(TaskOperation.CREATE_BM25_INDEX) as step:
                await self._create_bm25_index(index.snippets)

            # Create code embeddings
            async with operation.create_child(
                TaskOperation.CREATE_CODE_EMBEDDINGS
            ) as step:
                await self._create_code_embeddings(index.snippets, step)

            # Enrich snippets
            async with operation.create_child(TaskOperation.ENRICH_SNIPPETS) as step:
                enriched_snippets = (
                    await self.index_domain_service.enrich_snippets_in_index(
                        snippets=index.snippets,
                        reporting_step=step,
                    )
                )
                # Update snippets in repository
                await self.index_repository.update_snippets(index.id, enriched_snippets)

            # Create text embeddings (on enriched content)
            async with operation.create_child(
                TaskOperation.CREATE_TEXT_EMBEDDINGS
            ) as step:
                await self._create_text_embeddings(enriched_snippets, step)

            # Update index timestamp
            async with operation.create_child(
                TaskOperation.UPDATE_INDEX_TIMESTAMP
            ) as step:
                await self.index_repository.update_index_timestamp(index.id)

            # After indexing, clear the file processing statuses
            async with operation.create_child(
                TaskOperation.CLEAR_FILE_PROCESSING_STATUSES
            ) as step:
                index.source.working_copy.clear_file_processing_statuses()
                await self.index_repository.update(index)

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
    async def _create_bm25_index(self, snippets: list[Snippet]) -> None:
        await self.bm25_service.index_documents(
            IndexRequest(
                documents=[
                    Document(snippet_id=snippet.id, text=snippet.original_text())
                    for snippet in snippets
                    if snippet.id
                ]
            )
        )

    async def _create_code_embeddings(
        self, snippets: list[Snippet], reporting_step: ProgressTracker
    ) -> None:
        await reporting_step.set_total(len(snippets))
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
            await reporting_step.set_current(
                processed, f"Creating code embeddings for {processed} snippets"
            )

    async def _create_text_embeddings(
        self, snippets: list[Snippet], reporting_step: ProgressTracker
    ) -> None:
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
            await reporting_step.skip(
                "No snippets with summaries to create text embeddings"
            )
            return

        await reporting_step.set_total(len(documents_with_summaries))
        processed = 0
        async for result in self.text_search_service.index_documents(
            IndexRequest(documents=documents_with_summaries)
        ):
            processed += len(result)
            await reporting_step.set_current(
                processed, f"Creating text embeddings for {processed} snippets"
            )

    async def delete_index(self, index: Index) -> None:
        """Delete an index."""
        # Delete the index from the domain
        await self.index_domain_service.delete_index(index)

        # Delete index from the database
        await self.index_repository.delete(index)
