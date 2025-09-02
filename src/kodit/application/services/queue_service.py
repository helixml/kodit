"""Queue service for managing tasks."""

import structlog

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
        task_repository: TaskRepository,
    ) -> None:
        """Initialize the queue service."""
        self.task_repository = task_repository
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

    async def list_tasks(self, task_type: TaskType | None = None) -> list[Task]:
        """List all tasks in the queue."""
        return await self.task_repository.list(task_type)

    async def get_task(self, task_id: str) -> Task | None:
        """Get a specific task by ID."""
        return await self.task_repository.get(task_id)
