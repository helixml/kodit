"""Unified application service for code indexing operations."""

from datetime import UTC, datetime
from pathlib import Path

import structlog

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
from kodit.domain.services.snippet_service import (
    SnippetDomainService,
)
from kodit.domain.value_objects import (
    IndexRequest,
    MultiSearchRequest,
    MultiSearchResult,
    QueuePriority,
    SnippetSearchFilters,
    TrackableType,
)
from kodit.domain.value_objects import TaskOperation as DomainTaskOperation
from kodit.log import log_event

INDEXING_TASK_LIST = [
    DomainTaskOperation.REFRESH_WORKING_COPY,
    DomainTaskOperation.EXTRACT_SNIPPETS,
    DomainTaskOperation.CREATE_BM25_INDEX,
    DomainTaskOperation.CREATE_CODE_EMBEDDINGS,
    DomainTaskOperation.ENRICH_SNIPPETS,
]


class CodeIndexingApplicationService:
    """Unified application service for all code indexing operations."""

    # List of tasks that form an indexing pipeline. Order is important.

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
        queue_service: QueueService,
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
        self.log = structlog.get_logger(__name__)
        self.queue = queue_service
        self.snippet_domain_service = SnippetDomainService()

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
        documents = self.snippet_domain_service.prepare_documents_for_indexing(snippets)
        await self.bm25_service.index_documents(IndexRequest(documents=documents))

    async def _create_code_embeddings(
        self, snippets: list[Snippet], reporting_step: ProgressTracker
    ) -> None:
        documents = self.snippet_domain_service.prepare_documents_for_indexing(snippets)
        await reporting_step.set_total(len(snippets))
        processed = 0
        async for result in self.code_search_service.index_documents(
            IndexRequest(documents=documents)
        ):
            processed += len(result)
            await reporting_step.set_current(
                processed, f"Creating code embeddings for {processed} snippets"
            )

    async def _create_text_embeddings(
        self, snippets: list[Snippet], reporting_step: ProgressTracker
    ) -> None:
        # Prepare summary documents using domain service
        documents_with_summaries = (
            self.snippet_domain_service.prepare_summary_documents(snippets)
        )

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
        priority_offset = len(INDEXING_TASK_LIST) * 10
        for task in INDEXING_TASK_LIST:
            await self.queue.enqueue_task(
                Task.create(task, base + priority_offset, {"index_id": index_id})
            )
            priority_offset -= 10

    async def run_index_tasks_sync(self, index: Index) -> None:
        """Run all indexing phases synchronously."""
        if not index.id:
            raise ValueError("Index must have an ID")

        # Run all phases sequentially
        for task in INDEXING_TASK_LIST:
            await self.run_task(
                Task.create(task, QueuePriority.USER_INITIATED, {"index_id": index.id})
            )

    async def process_refresh_working_copy(self, index_id: int) -> None:
        """Refresh working copy of the index."""
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

    async def process_extract(self, index_id: int) -> None:
        """Extract snippets from changed files."""
        index = await self.index_repository.get(index_id)
        if not index:
            raise ValueError(f"Index not found: {index_id}")

        async with self.operation.create_child(
            TaskOperation.EXTRACT_SNIPPETS,
            trackable_type=TrackableType.INDEX,
            trackable_id=index_id,
        ) as operation:
            changed_files = index.source.working_copy.changed_files()
            changed_file_ids = [f.id for f in changed_files if f.id]
            if len(changed_file_ids) == 0:
                await operation.skip("No changed files")
                return

            # Extract new snippets from changed files
            extracted_snippets = (
                await self.index_domain_service.extract_snippets_from_index(
                    index=index, step=operation
                )
            )

            # Get existing snippets for changed files to compare content hashes
            existing_snippet_contexts = (
                await self.snippet_repository.get_by_file_ids(changed_file_ids)
                if changed_file_ids
                else []
            )
            existing_snippets = [sc.snippet for sc in existing_snippet_contexts]

            # Analyze snippet changes using domain service
            change_analysis = self.snippet_domain_service.analyze_snippet_changes(
                existing_snippets, extracted_snippets
            )
            snippets_to_add = change_analysis.snippets_to_add
            snippet_ids_to_delete = change_analysis.snippet_ids_to_delete

            # Perform efficient database operations
            if snippet_ids_to_delete:
                async with operation.create_child(TaskOperation.DELETE_OLD_SNIPPETS):
                    await self.snippet_repository.delete_by_ids(snippet_ids_to_delete)

            # Add new and modified snippets
            if snippets_to_add:
                await self.snippet_repository.add(snippets_to_add, index.id)

            # Persist updated index metadata
            await self.index_repository.update(index)

    async def process_bm25_index(self, index_id: int) -> None:
        """Handle BM25_INDEX task - create keyword index INCREMENTALLY."""
        async with self.operation.create_child(
            TaskOperation.CREATE_BM25_INDEX,
            trackable_type=TrackableType.INDEX,
            trackable_id=index_id,
        ) as step:
            # ENHANCED: Get only snippets needing BM25 processing
            snippets_needing_processing = (
                await self.snippet_repository.get_snippets_needing_processing(
                    index_id, TaskOperation.CREATE_BM25_INDEX
                )
            )
            pending_snippets = [sc.snippet for sc in snippets_needing_processing]

            if not pending_snippets:
                await step.skip("All snippets already have BM25 index")
                return

            await self._create_bm25_index(pending_snippets)

            # ENHANCED: Mark processing state as completed
            await self.snippet_repository.mark_processing_completed(
                [s.id for s in pending_snippets if s.id],
                TaskOperation.CREATE_BM25_INDEX,
            )

    async def process_code_embeddings(self, index_id: int) -> None:
        """Handle CODE_EMBEDDINGS task - create code embeddings INCREMENTALLY."""
        async with self.operation.create_child(
            TaskOperation.CREATE_CODE_EMBEDDINGS,
            trackable_type=TrackableType.INDEX,
            trackable_id=index_id,
        ) as step:
            # ENHANCED: Get only snippets needing code embeddings
            snippets_needing_processing = (
                await self.snippet_repository.get_snippets_needing_processing(
                    index_id, TaskOperation.CREATE_CODE_EMBEDDINGS
                )
            )
            pending_snippets = [sc.snippet for sc in snippets_needing_processing]

            if not pending_snippets:
                await step.skip("All snippets already have code embeddings")
                return

            await self._create_code_embeddings(pending_snippets, step)

            # ENHANCED: Mark processing state as completed
            await self.snippet_repository.mark_processing_completed(
                [s.id for s in pending_snippets if s.id],
                TaskOperation.CREATE_CODE_EMBEDDINGS,
            )

    async def process_enrich(self, index_id: int) -> None:
        """Enrich snippets incrementally."""
        index = await self.index_repository.get(index_id)
        if not index:
            raise ValueError(f"Index not found: {index_id}")

        async with self.operation.create_child(
            TaskOperation.ENRICH_SNIPPETS,
            trackable_type=TrackableType.INDEX,
            trackable_id=index_id,
        ) as operation:
            # ENHANCED: Get only snippets needing enrichment
            snippets_needing_processing = (
                await self.snippet_repository.get_snippets_needing_processing(
                    index_id, TaskOperation.ENRICH_SNIPPETS
                )
            )
            pending_snippets = [sc.snippet for sc in snippets_needing_processing]

            if not pending_snippets:
                self.log.info("No snippets need enrichment", index_id=index_id)
            else:
                # Enrich snippets
                enriched_snippets = (
                    await self.index_domain_service.enrich_snippets_in_index(
                        snippets=pending_snippets,
                        reporting_step=operation,
                    )
                )
                await self.snippet_repository.update(enriched_snippets)

                # ENHANCED: Mark enrichment processing as completed
                await self.snippet_repository.mark_processing_completed(
                    [s.id for s in enriched_snippets if s.id],
                    TaskOperation.ENRICH_SNIPPETS,
                )

            # Create text embeddings for snippets needing them
            text_embeddings_needing_processing = (
                await self.snippet_repository.get_snippets_needing_processing(
                    index_id, TaskOperation.CREATE_TEXT_EMBEDDINGS
                )
            )
            text_pending_snippets = [
                sc.snippet for sc in text_embeddings_needing_processing
            ]

            if text_pending_snippets:
                async with operation.create_child(
                    TaskOperation.CREATE_TEXT_EMBEDDINGS
                ) as step:
                    await self._create_text_embeddings(text_pending_snippets, step)

                    # ENHANCED: Mark text embeddings processing as completed
                    await self.snippet_repository.mark_processing_completed(
                        [s.id for s in text_pending_snippets if s.id],
                        TaskOperation.CREATE_TEXT_EMBEDDINGS,
                    )

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

    async def run_task(self, task: Task) -> None:
        """Run a task."""
        index_id = task.payload["index_id"]
        if task.type == TaskOperation.REFRESH_WORKING_COPY:
            await self.process_refresh_working_copy(index_id)
        elif task.type == TaskOperation.EXTRACT_SNIPPETS:
            await self.process_extract(index_id)
        elif task.type == TaskOperation.CREATE_BM25_INDEX:
            await self.process_bm25_index(index_id)
        elif task.type == TaskOperation.CREATE_CODE_EMBEDDINGS:
            await self.process_code_embeddings(index_id)
        elif task.type == TaskOperation.ENRICH_SNIPPETS:
            await self.process_enrich(index_id)
        else:
            raise ValueError(f"Unknown task type: {task.type}")
