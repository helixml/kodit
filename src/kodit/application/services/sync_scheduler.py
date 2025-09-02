"""Service for scheduling periodic sync operations."""

import asyncio
from contextlib import suppress

import structlog

from kodit.application.services.queue_service import QueueService
from kodit.domain.entities import Task
from kodit.domain.protocols import IndexRepository, TaskRepository, UnitOfWork
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.value_objects import QueuePriority
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.sqlalchemy.repository_factories import (
    create_index_repository,
    create_task_repository,
)


class SyncSchedulerService:
    """Service for scheduling periodic sync operations."""

    def __init__(
        self,
        index_repository: IndexRepository,
        task_repository: TaskRepository,
    ) -> None:
        """Initialize the sync scheduler service."""
        self.index_repository = index_repository
        self.task_repository = task_repository
        self.log = structlog.get_logger(__name__)
        self._sync_task: asyncio.Task | None = None
        self._shutdown_event = asyncio.Event()

    def start_periodic_sync(self, interval_seconds: float = 1800) -> None:
        """Start periodic sync of all indexes."""
        self.log.info("Starting periodic sync", interval_seconds=interval_seconds)

        self._sync_task = asyncio.create_task(self._sync_loop(interval_seconds))

    async def stop_periodic_sync(self) -> None:
        """Stop the periodic sync task."""
        self.log.info("Stopping periodic sync")
        self._shutdown_event.set()

        if self._sync_task and not self._sync_task.done():
            self._sync_task.cancel()
            with suppress(asyncio.CancelledError):
                await self._sync_task

    async def _sync_loop(self, interval_seconds: float) -> None:
        """Run the sync loop at the specified interval."""
        while not self._shutdown_event.is_set():
            try:
                await self._perform_sync()
            except Exception as e:
                self.log.exception("Sync operation failed", error=e)

            # Wait for the interval or until shutdown
            try:
                await asyncio.wait_for(
                    self._shutdown_event.wait(), timeout=interval_seconds
                )
                # If we reach here, shutdown was requested
                break
            except TimeoutError:
                # Continue to next sync cycle
                continue

    async def _perform_sync(self) -> None:
        """Perform a sync operation on all indexes."""
        self.log.info("Starting sync operation")

        # Create services
        queue_service = QueueService(task_repository=self.task_repository)
        index_query_service = IndexQueryService(
            index_repository=self.index_repository,
            fusion_service=ReciprocalRankFusionService(),
        )

        # Get all existing indexes
        all_indexes = await index_query_service.list_indexes()

        if not all_indexes:
            self.log.info("No indexes found to sync")
            return

        self.log.info("Adding sync tasks to queue", count=len(all_indexes))

        # Sync each index
        for index in all_indexes:
            await queue_service.enqueue_task(
                Task.create_index_update_task(index.id, QueuePriority.BACKGROUND)
            )

        self.log.info("Sync operation completed")


def create_sync_scheduler_service(
    uow: UnitOfWork,
) -> SyncSchedulerService:
    """Create a sync scheduler service."""
    index_repository = create_index_repository(uow)
    task_repository = create_task_repository(uow)
    return SyncSchedulerService(index_repository, task_repository)
