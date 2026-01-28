"""Queue service for managing tasks."""

from collections.abc import Callable
from typing import Any

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Task
from kodit.domain.value_objects import QueuePriority, TaskOperation
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder
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
        # Check if task already exists by dedup_key (upsert semantics)
        existing, created = await self.task_repository.get_or_create(task, "dedup_key")
        if not created:
            # Task already exists - update priority if needed
            existing.priority = task.priority
            await self.task_repository.save(existing)
            self.log.info(
                "Task updated", task_dedup_key=task.dedup_key, task_type=task.type
            )
        else:
            self.log.info(
                "Task queued",
                task_dedup_key=task.dedup_key,
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
        query = QueryBuilder()
        if task_operation:
            query.filter("type", FilterOperator.EQ, task_operation.value)
        query.sort("priority", descending=True)
        query.sort("created_at", descending=True)
        return await self.task_repository.find(query)

    async def get_task(self, dedup_key: str) -> Task | None:
        """Get a specific task by dedup_key."""
        query = QueryBuilder().filter("dedup_key", FilterOperator.EQ, dedup_key)
        tasks = await self.task_repository.find(query)
        return tasks[0] if tasks else None
