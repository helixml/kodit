"""Task repository for the task queue."""

from collections.abc import Callable

import structlog
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Task
from kodit.domain.protocols import TaskRepository
from kodit.domain.value_objects import TaskOperation
from kodit.infrastructure.mappers.task_mapper import TaskMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_task_repository(
    session_factory: Callable[[], AsyncSession],
) -> TaskRepository:
    """Create an index repository."""
    return SqlAlchemyTaskRepository(session_factory=session_factory)


class SqlAlchemyTaskRepository(TaskRepository):
    """Repository for task persistence using the existing Task entity."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory
        self.log = structlog.get_logger(__name__)

    async def add(
        self,
        task: Task,
    ) -> None:
        """Create a new task in the database."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            session.add(TaskMapper.from_domain_task(task))

    async def get(self, task_id: str) -> Task | None:
        """Get a task by ID."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.Task).where(db_entities.Task.dedup_key == task_id)
            result = await session.execute(stmt)
            db_task = result.scalar_one_or_none()
            if not db_task:
                return None
            return TaskMapper.to_domain_task(db_task)

    async def next(self) -> Task | None:
        """Take a task for processing and remove it from the database."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = (
                select(db_entities.Task)
                .order_by(db_entities.Task.priority.desc(), db_entities.Task.created_at)
                .limit(1)
            )
            result = await session.execute(stmt)
            db_task = result.scalar_one_or_none()
            if not db_task:
                return None
            return TaskMapper.to_domain_task(db_task)

    async def remove(self, task: Task) -> None:
        """Remove a task from the database."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_task = await session.scalar(
                select(db_entities.Task).where(db_entities.Task.dedup_key == task.id)
            )
            if not db_task:
                raise ValueError(f"Task not found: {task.id}")
            await session.delete(db_task)

    async def update(self, task: Task) -> None:
        """Update a task in the database."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.Task).where(db_entities.Task.dedup_key == task.id)
            result = await session.execute(stmt)
            db_task = result.scalar_one_or_none()

            if not db_task:
                raise ValueError(f"Task not found: {task.id}")

            db_task.priority = task.priority
            db_task.payload = task.payload

    async def list(self, task_operation: TaskOperation | None = None) -> list[Task]:
        """List tasks with optional status filter."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.Task)

            if task_operation:
                stmt = stmt.where(db_entities.Task.type == task_operation.value)

            stmt = stmt.order_by(
                db_entities.Task.priority.desc(), db_entities.Task.created_at
            )

            result = await session.execute(stmt)
            records = result.scalars().all()

            # Convert to domain entities
            return [TaskMapper.to_domain_task(record) for record in records]
