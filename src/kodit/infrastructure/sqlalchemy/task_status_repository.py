"""Task repository for the task queue."""

from collections.abc import Callable

import structlog
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain import entities as domain_entities
from kodit.domain.protocols import TaskStatusRepository
from kodit.infrastructure.mappers.task_status_mapper import TaskStatusMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_task_status_repository(
    session_factory: Callable[[], AsyncSession],
) -> TaskStatusRepository:
    """Create an index repository."""
    return SqlAlchemyTaskStatusRepository(session_factory=session_factory)


class SqlAlchemyTaskStatusRepository(TaskStatusRepository):
    """Repository for persisting TaskStatus entities."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory
        self.mapper = TaskStatusMapper()
        self.log = structlog.get_logger(__name__)

    async def save(self, status: domain_entities.TaskStatus) -> None:
        """Save a TaskStatus to database."""
        # If this task has a parent, ensure the parent exists in the database first
        if status.parent is not None:
            async with SqlAlchemyUnitOfWork(self.session_factory) as session:
                parent_stmt = select(db_entities.TaskStatus).where(
                    db_entities.TaskStatus.id == status.parent.id,
                )
                parent_result = await session.execute(parent_stmt)
                existing_parent = parent_result.scalar_one_or_none()

            if not existing_parent:
                # Recursively save the parent first
                await self.save(status.parent)

        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Convert domain entity to database entity
            db_status = self.mapper.from_domain_task_status(status)
            stmt = select(db_entities.TaskStatus).where(
                db_entities.TaskStatus.id == db_status.id,
            )
            result = await session.execute(stmt)
            existing = result.scalar_one_or_none()

            if not existing:
                session.add(db_status)
            else:
                # Update existing record with new values
                existing.operation = db_status.operation
                existing.state = db_status.state
                existing.error = db_status.error
                existing.total = db_status.total
                existing.current = db_status.current
                existing.updated_at = db_status.updated_at
                existing.parent = db_status.parent
                existing.trackable_id = db_status.trackable_id
                existing.trackable_type = db_status.trackable_type

    async def load_with_hierarchy(
        self, trackable_type: str, trackable_id: int
    ) -> list[domain_entities.TaskStatus]:
        """Load TaskStatus entities with hierarchy from database."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.TaskStatus).where(
                db_entities.TaskStatus.trackable_id == trackable_id,
                db_entities.TaskStatus.trackable_type == trackable_type,
            )
            result = await session.execute(stmt)
            db_statuses = list(result.scalars().all())

            # Use mapper to convert and reconstruct hierarchy
            return self.mapper.to_domain_task_status_with_hierarchy(db_statuses)

    async def delete(self, status: domain_entities.TaskStatus) -> None:
        """Delete a TaskStatus."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = delete(db_entities.TaskStatus).where(
                db_entities.TaskStatus.id == status.id,
            )
            await session.execute(stmt)
