"""Service for processing indexing tasks from the database queue."""

import asyncio
from collections.abc import Callable
from concurrent.futures import ThreadPoolExecutor
from contextlib import suppress
from datetime import UTC, datetime

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.code_indexing_factory import (
    create_code_indexing_application_service,
)
from kodit.config import AppContext
from kodit.domain.entities import Task
from kodit.domain.value_objects import TaskType
from kodit.infrastructure.sqlalchemy.task_repository import SqlAlchemyTaskRepository


class IndexingWorkerService:
    """Service for processing indexing tasks from the database queue.

    This worker polls the database for pending tasks and processes the heavy
    indexing work in separate threads to prevent blocking API responsiveness.
    """

    def __init__(
        self,
        app_context: AppContext,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Initialize the indexing worker service."""
        self.app_context = app_context
        self.session_factory = session_factory
        self._worker_task: asyncio.Task | None = None
        self._shutdown_event = asyncio.Event()
        self._executor = ThreadPoolExecutor(
            max_workers=1, thread_name_prefix="indexing-worker"
        )
        self.log = structlog.get_logger(__name__)

    async def start(self) -> None:
        """Start the worker to process the queue."""
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
            self._worker_task.cancel()
            with suppress(asyncio.CancelledError):
                await self._worker_task

        # Shutdown the thread pool executor
        self._executor.shutdown(wait=True)

        self.log.info("Indexing worker stopped")

    async def _worker_loop(self) -> None:
        self.log.debug("Worker loop started")

        while not self._shutdown_event.is_set():
            try:
                async with self.session_factory() as session:
                    repo = SqlAlchemyTaskRepository(session)
                    task = await repo.take()
                    await session.commit()

                # If there's a task, process it in a new thread
                if task:
                    await asyncio.get_event_loop().run_in_executor(
                        self._executor, self._process_task, task
                    )
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

    def _process_task(self, task: Task) -> None:
        """Process a single task."""
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
            # Process based on task type (currently only INDEX_UPDATE is supported)
            if task.type is TaskType.INDEX_UPDATE:
                loop.run_until_complete(self._process_index_update(task))
            else:
                self.log.warning(
                    "Unknown task type",
                    task_id=task.id,
                    task_type=task.type,
                )
                return
        finally:
            loop.close()

        duration = (datetime.now(UTC) - start_time).total_seconds()
        self.log.info(
            "Task completed successfully",
            task_id=task.id,
            duration_seconds=duration,
        )

    async def _process_index_update(self, task: Task) -> None:
        """Process index update/sync task."""
        index_id = task.payload.get("index_id")
        if not index_id:
            raise ValueError("Missing index_id in task payload")

        # Create a fresh database connection for this thread's event loop
        db = await self.app_context.new_db(run_migrations=True)
        try:
            async with db.session_factory() as session:
                service = create_code_indexing_application_service(
                    app_context=self.app_context,
                    session=session,
                )
                index = await service.index_repository.get(index_id)
                if not index:
                    raise ValueError(f"Index not found: {index_id}")

                await service.run_index(index)
        finally:
            await db.close()
