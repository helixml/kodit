"""Queue service for managing tasks."""

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Task
from kodit.domain.value_objects import TaskType
from kodit.infrastructure.sqlalchemy.task_repository import SqlAlchemyTaskRepository


class QueueService:
    """Service for queue operations using database persistence.

    This service provides the main interface for enqueuing and managing tasks.
    It uses the existing Task entity in the database with a flexible JSON payload.
    """

    def __init__(
        self,
        session: AsyncSession,
    ) -> None:
        """Initialize the queue service."""
        self.session = session
        self.log = structlog.get_logger(__name__)

    async def enqueue_task(self, task: Task) -> None:
        """Queue a task in the database."""
        repo = SqlAlchemyTaskRepository(self.session)

        # See if task already exists
        db_task = await repo.get(task.id)
        if db_task:
            # Task already exists, update priority
            db_task.priority = task.priority
            await repo.update(db_task)
            self.log.info("Task updated", task_id=task.id, task_type=task.type)
        else:
            # Otherwise, add task
            await repo.add(task)
            self.log.info(
                "Task queued",
                task_id=task.id,
                task_type=task.type,
                payload=task.payload,
            )

        await self.session.commit()

    async def list_tasks(self, task_type: TaskType | None = None) -> list[Task]:
        """List all tasks in the queue."""
        repo = SqlAlchemyTaskRepository(self.session)
        return await repo.list(task_type)

    async def get_task(self, task_id: str) -> Task | None:
        """Get a specific task by ID."""
        repo = SqlAlchemyTaskRepository(self.session)
        return await repo.get(task_id)
