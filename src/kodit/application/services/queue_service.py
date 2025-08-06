from collections.abc import Callable
from typing import Any

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Task
from kodit.domain.protocols import TaskRepository
from kodit.domain.value_objects import TaskType


class QueueService:
    """Service for queue operations using database persistence.

    This service provides the main interface for enqueuing and managing tasks.
    It uses the existing Task entity in the database with a flexible JSON payload.
    """

    def __init__(
        self,
        session_factory: Callable[[], AsyncSession],
        task_repository: TaskRepository,
    ) -> None:
        """Initialize the queue service."""
        self.session_factory = session_factory
        self.task_repository = task_repository
        self.log = structlog.get_logger(__name__)

    async def enqueue_task(
        self, task_type: TaskType, priority: int, payload: dict[str, Any]
    ) -> None:
        """Queue a task in the database."""
        async with self.session_factory() as session:
            # Create task using factory method
            task = Task.create(task_type, priority, payload)

            # See if task already exists
            if await self.task_repository.get(task.id):
                # Task already exists, update priority
                task.priority = priority
                await self.task_repository.update(task)
                self.log.info(
                    "Task updated", task_id=task.id, task_type=task_type.value
                )
            else:
                # Otherwise, add task
                await self.task_repository.add(task)
                self.log.info("Task queued", task_id=task.id, task_type=task_type.value)

            await session.commit()

    async def list_tasks(self, task_type: TaskType | None = None) -> list[Task]:
        """List all tasks in the queue."""
        return await self.task_repository.list(task_type)
