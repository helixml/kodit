"""Queue service for managing tasks."""

from collections.abc import Callable
from typing import Any

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Task
from kodit.domain.value_objects import QueuePriority, TaskOperation
from kodit.infrastructure.sqlalchemy.task_repository import (
    create_task_repository,
)


class QueueService:
    """Service for queue operations using database persistence.

    This service provides the main interface for enqueuing and managing tasks.
    It uses the existing Task entity in the database with a flexible JSON payload.
    """

    def __init__(
        self,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Initialize the queue service."""
        self.task_repository = create_task_repository(session_factory=session_factory)
        self.log = structlog.get_logger(__name__)

    async def enqueue_task(self, task: Task) -> None:
        """Queue a task in the database."""
        # See if task already exists
        db_task = await self.task_repository.get(task.id)
        if db_task:
            # Task already exists, update priority
            db_task.priority = task.priority
            await self.task_repository.update(db_task)
            self.log.info("Task updated", task_id=task.id, task_type=task.type)
        else:
            # Otherwise, add task
            await self.task_repository.add(task)
            self.log.info(
                "Task queued",
                task_id=task.id,
                task_type=task.type,
                payload=task.payload,
            )

    async def enqueue_tasks(
        self,
        tasks: list[TaskOperation],
        base_priority: QueuePriority,
        payload: dict[str, Any],
    ) -> None:
        """Queue repository tasks."""
        priority_offset = len(tasks) * 10
        for task in tasks:
            await self.enqueue_task(
                Task.create(
                    task,
                    base_priority + priority_offset,
                    payload,
                )
            )
            priority_offset -= 10

    async def list_tasks(
        self, task_operation: TaskOperation | None = None
    ) -> list[Task]:
        """List all tasks in the queue."""
        return await self.task_repository.list(task_operation)

    async def get_task(self, task_id: str) -> Task | None:
        """Get a specific task by ID."""
        return await self.task_repository.get(task_id)
