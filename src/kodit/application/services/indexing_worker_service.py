"""Service for processing indexing tasks from the database queue."""

import asyncio
from collections.abc import Callable
from contextlib import suppress
from datetime import UTC, datetime

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.reporting_factory import create_noop_operation
from kodit.application.factories.server_factory import ServerFactory
from kodit.application.services.reporting import ProgressTracker
from kodit.config import AppContext
from kodit.domain.entities import Task
from kodit.infrastructure.sqlalchemy.task_repository import create_task_repository


class IndexingWorkerService:
    """Service for processing indexing tasks from the database queue.

    This worker polls the database for pending tasks and processes the heavy
    indexing work in separate threads to prevent blocking API responsiveness.
    """

    def __init__(
        self,
        app_context: AppContext,
        session_factory: Callable[[], AsyncSession],
        server_factory: ServerFactory,
    ) -> None:
        """Initialize the indexing worker service."""
        self.app_context = app_context
        self.session_factory = session_factory
        self.server_factory = server_factory
        self._worker_task: asyncio.Task | None = None
        self._shutdown_event = asyncio.Event()
        self.task_repository = create_task_repository(session_factory)
        self.log = structlog.get_logger(__name__)

    async def start(self, operation: ProgressTracker | None = None) -> None:
        """Start the worker to process the queue."""
        operation = operation or create_noop_operation()
        self._running = True

        # Start single worker task
        self._worker_task = asyncio.create_task(self._worker_loop())

        self.log.info(
            "Indexing worker started",
        )

    async def stop(self) -> None:
        """Stop the worker gracefully."""
        self.log.info("Stopping indexing worker")
        self._shutdown_event.set()

        if self._worker_task and not self._worker_task.done():
            with suppress(asyncio.CancelledError):
                self._worker_task.cancel()
                await self._worker_task

        self.log.info("Indexing worker stopped")

    async def _worker_loop(self) -> None:
        self.log.debug("Worker loop started")

        while not self._shutdown_event.is_set():
            try:
                async with self.session_factory() as session:
                    task = await self.task_repository.next()
                    await session.commit()

                # If there's a task, process it in a new thread
                if task:
                    await self._process_task(task)
                    # Only remove the task if it was processed successfully
                    await self.task_repository.delete(task)
                    continue

                # If no task, sleep for a bit
                await asyncio.sleep(1)
                continue

            except Exception as e:
                self.log.exception(
                    "Error processing task",
                    error=str(e),
                )
                continue

        self.log.info("Worker loop stopped")

    async def _process_task(self, task: Task) -> None:
        """Process a task based on its type."""
        self.log.info(
            "Processing task",
            task_id=task.id,
            task_type=task.type.value,
        )

        start_time = datetime.now(UTC)

        # Create a new event loop for this thread
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)

        try:
            commit_service = self.server_factory.commit_indexing_application_service()
            await commit_service.run_task(task)
        finally:
            loop.close()

        duration = (datetime.now(UTC) - start_time).total_seconds()
        self.log.info(
            "Task completed successfully",
            task_id=task.id,
            duration_seconds=duration,
        )
