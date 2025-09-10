"""Unified application service for code indexing operations."""

from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import (
    ProgressTracker,
    TaskOperation,
)
from kodit.domain.entities import Index, Snippet, Task
from kodit.domain.protocols import IndexRepository, SnippetRepository
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
    QueuePriority,
    SnippetSearchFilters,
    TrackableType,
)
from kodit.domain.value_objects import TaskOperation as DomainTaskOperation
from kodit.log import log_event


class CodeIndexingApplicationService:
    """Unified application service for all code indexing operations."""

    def __init__(  # noqa: PLR0913
        self,
        indexing_domain_service: IndexDomainService,
        index_repository: IndexRepository,
        snippet_repository: SnippetRepository,
        index_query_service: IndexQueryService,
        bm25_service: BM25DomainService,
        code_search_service: EmbeddingDomainService,
        text_search_service: EmbeddingDomainService,
        enrichment_service: EnrichmentDomainService,
        operation: ProgressTracker,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Initialize the code indexing application service."""
        self.index_domain_service = indexing_domain_service
        self.index_repository = index_repository
        self.snippet_repository = snippet_repository
        self.index_query_service = index_query_service
        self.bm25_service = bm25_service
        self.code_search_service = code_search_service
        self.text_search_service = text_search_service
        self.enrichment_service = enrichment_service
        self.operation = operation
        self.session_factory = session_factory
        self.log = structlog.get_logger(__name__)
        self.queue = QueueService(self.session_factory)

    async def does_index_exist(self, uri: str) -> bool:
        """Check if an index exists for a source."""
        # Check if index already exists
        sanitized_uri, _ = self.index_domain_service.sanitize_uri(uri)
        existing_index = await self.index_repository.get_by_uri(sanitized_uri)
        return existing_index is not None

    async def create_index_from_uri(self, uri: str) -> Index:
        """Create a new index for a source."""
        if Path(uri).is_file():
            raise ValueError("Individual file indexing is not supported")

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

    async def queue_index_tasks(
        self, index_id: int, *, is_user_initiated: bool = True
    ) -> None:
        """Queue the 5 indexing tasks with priority ordering.

        This replaces the old run_index() method entirely.

        Args:
            index_id: The ID of the index to process
            is_user_initiated: True for API/CLI calls, False for background syncs

        """
        # Use different base priority for user vs background tasks
        base = (
            QueuePriority.USER_INITIATED
            if is_user_initiated
            else QueuePriority.BACKGROUND
        )

        # Queue tasks with descending priority to ensure execution order
        await self.queue.enqueue_task(
            Task.create(
                DomainTaskOperation.REFRESH_WORKING_COPY,
                base + 40,
                {"index_id": index_id},
            )
        )
        await self.queue.enqueue_task(
            Task.create(
                DomainTaskOperation.EXTRACT_SNIPPETS, base + 30, {"index_id": index_id}
            )
        )
        await self.queue.enqueue_task(
            Task.create(
                DomainTaskOperation.CREATE_BM25_INDEX, base + 20, {"index_id": index_id}
            )
        )
        await self.queue.enqueue_task(
            Task.create(
                DomainTaskOperation.CREATE_CODE_EMBEDDINGS,
                base + 10,
                {"index_id": index_id},
            )
        )
        await self.queue.enqueue_task(
            Task.create(
                DomainTaskOperation.ENRICH_SNIPPETS, base, {"index_id": index_id}
            )
        )

    async def run_index_tasks_sync(self, index: Index) -> None:
        """Run all indexing phases synchronously."""
        if not index.id:
            raise ValueError("Index must have an ID")

        # Run all phases sequentially
        await self.process_sync(index.id)
        await self.process_extract(index.id)
        await self.process_bm25_index(index.id)
        await self.process_code_embeddings(index.id)
        await self.process_enrich(index.id)

    async def process_sync(self, index_id: int) -> None:
        """Handle SYNC task - refresh working copy."""
        index = await self.index_repository.get(index_id)
        if not index:
            raise ValueError(f"Index not found: {index_id}")

        async with self.operation.create_child(
            TaskOperation.REFRESH_WORKING_COPY,
            trackable_type=TrackableType.INDEX,
            trackable_id=index_id,
        ) as step:
            index.source.working_copy = (
                await self.index_domain_service.refresh_working_copy(
                    index.source.working_copy, step
                )
            )
            await self.index_repository.update(index)

            if len(index.source.working_copy.changed_files()) == 0:
                self.log.info("No new changes to index", index_id=index_id)
                await step.skip("No new changes to index")
                # Don't queue further tasks if no changes
                return

    async def process_extract(self, index_id: int) -> None:
        """Handle EXTRACT task - extract snippets from changed files."""
        index = await self.index_repository.get(index_id)
        if not index:
            raise ValueError(f"Index not found: {index_id}")

        # Safety check: ensure we have changed files to process
        if len(index.source.working_copy.changed_files()) == 0:
            self.log.info("No files to extract", index_id=index_id)
            return

        async with self.operation.create_child(
            TaskOperation.EXTRACT_SNIPPETS,
            trackable_type=TrackableType.INDEX,
            trackable_id=index_id,
        ) as operation:
            # Delete old snippets
            async with operation.create_child(TaskOperation.DELETE_OLD_SNIPPETS):
                await self.snippet_repository.delete_by_file_ids(
                    [f.id for f in index.source.working_copy.changed_files() if f.id]
                )

            # Extract new snippets
            extracted_snippets = (
                await self.index_domain_service.extract_snippets_from_index(
                    index=index, step=operation
                )
            )

            # Persist files and snippets
            await self.index_repository.update(index)
            if extracted_snippets and index.id:
                await self.snippet_repository.add(extracted_snippets, index.id)

    async def process_bm25_index(self, index_id: int) -> None:
        """Handle BM25_INDEX task - create keyword index."""
        async with self.operation.create_child(
            TaskOperation.CREATE_BM25_INDEX,
            trackable_type=TrackableType.INDEX,
            trackable_id=index_id,
        ):
            snippets = await self.snippet_repository.get_by_index_id(index_id)
            snippet_list = [sc.snippet for sc in snippets]

            if not snippet_list:
                self.log.info("No snippets to index", index_id=index_id)
                return

            await self._create_bm25_index(snippet_list)

    async def process_code_embeddings(self, index_id: int) -> None:
        """Handle CODE_EMBEDDINGS task - create code embeddings."""
        async with self.operation.create_child(
            TaskOperation.CREATE_CODE_EMBEDDINGS,
            trackable_type=TrackableType.INDEX,
            trackable_id=index_id,
        ) as step:
            snippets = await self.snippet_repository.get_by_index_id(index_id)
            snippet_list = [sc.snippet for sc in snippets]

            if not snippet_list:
                self.log.info("No snippets for embeddings", index_id=index_id)
                return

            await self._create_code_embeddings(snippet_list, step)

    async def process_enrich(self, index_id: int) -> None:
        """Handle ENRICH task - enrich snippets and create text embeddings."""
        index = await self.index_repository.get(index_id)
        if not index:
            raise ValueError(f"Index not found: {index_id}")

        async with self.operation.create_child(
            TaskOperation.ENRICH_SNIPPETS,
            trackable_type=TrackableType.INDEX,
            trackable_id=index_id,
        ) as operation:
            snippets = await self.snippet_repository.get_by_index_id(index_id)
            snippet_list = [sc.snippet for sc in snippets]

            if not snippet_list:
                self.log.info("No snippets to enrich", index_id=index_id)
                return

            # Enrich snippets
            enriched_snippets = (
                await self.index_domain_service.enrich_snippets_in_index(
                    snippets=snippet_list,
                    reporting_step=operation,
                )
            )
            await self.snippet_repository.update(enriched_snippets)

            # Create text embeddings
            async with operation.create_child(
                TaskOperation.CREATE_TEXT_EMBEDDINGS
            ) as step:
                await self._create_text_embeddings(enriched_snippets, step)

            # Update timestamp
            async with operation.create_child(
                TaskOperation.UPDATE_INDEX_TIMESTAMP
            ) as step:
                await self.index_repository.update_index_timestamp(index_id)

            # Clear file processing statuses
            async with operation.create_child(
                TaskOperation.CLEAR_FILE_PROCESSING_STATUSES
            ) as step:
                index.source.working_copy.clear_file_processing_statuses()
                await self.index_repository.update(index)

    async def delete_index(self, index: Index) -> None:
        """Delete an index."""
        # Delete the index from the domain
        await self.index_domain_service.delete_index(index)

        # Delete index from the database
        await self.index_repository.delete(index)
