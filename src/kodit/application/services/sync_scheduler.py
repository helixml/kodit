"""Service for scheduling periodic sync operations."""

import asyncio
from contextlib import suppress

import structlog

from kodit.application.services.queue_service import QueueService
from kodit.domain.protocols import GitRepoRepository
from kodit.domain.value_objects import (
    PrescribedOperations,
    QueuePriority,
)
from kodit.infrastructure.sqlalchemy.query import QueryBuilder


class SyncSchedulerService:
    """Service for scheduling periodic sync operations."""

    def __init__(
        self,
        queue_service: QueueService,
        repo_repository: GitRepoRepository,
    ) -> None:
        """Initialize the sync scheduler service."""
        self.queue_service = queue_service
        self.repo_repository = repo_repository
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

        # Sync each index - queue all 5 tasks with priority ordering
        for repo in await self.repo_repository.find(QueryBuilder()):
            await self.queue_service.enqueue_tasks(
                tasks=PrescribedOperations.SYNC_REPOSITORY,
                base_priority=QueuePriority.BACKGROUND,
                payload={"repository_id": repo.id},
            )

        self.log.info("Sync operation completed")
