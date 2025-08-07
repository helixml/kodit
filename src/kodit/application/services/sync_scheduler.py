"""Service for scheduling periodic sync operations."""

import asyncio
from collections.abc import Callable
from contextlib import suppress

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.queue_service import QueueService
from kodit.domain.entities import Task
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.value_objects import QueuePriority
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.sqlalchemy.index_repository import SqlAlchemyIndexRepository


class SyncSchedulerService:
    """Service for scheduling periodic sync operations."""

    def __init__(
        self,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Initialize the sync scheduler service."""
        self.session_factory = session_factory
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

        async with self.session_factory() as session:
            # Create services
            queue_service = QueueService(session=session)
            index_query_service = IndexQueryService(
                index_repository=SqlAlchemyIndexRepository(session=session),
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
