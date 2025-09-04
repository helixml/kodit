"""Task repository for the task queue."""

from collections.abc import Callable

import structlog
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.protocols import TaskStatusRepository
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_task_status_repository(
    session_factory: Callable[[], AsyncSession],
) -> TaskStatusRepository:
    """Create an index repository."""
    uow = SqlAlchemyUnitOfWork(session_factory=session_factory)
    return SqlAlchemyTaskStatusRepository(uow)


class SqlAlchemyTaskStatusRepository(TaskStatusRepository):
    """Repository for task persistence using the existing Task entity."""

    def __init__(self, uow: SqlAlchemyUnitOfWork) -> None:
        """Initialize the repository."""
        self.uow = uow
        self.log = structlog.get_logger(__name__)

    async def update(self, progress_tracker: ProgressTracker) -> None:
        """Create or update a task status."""
        status = await progress_tracker.status()
        async with self.uow:
            # See if this specific status exists already
            stmt = select(db_entities.TaskStatus).where(
                db_entities.TaskStatus.trackable_id == progress_tracker.trackable_id,
                db_entities.TaskStatus.trackable_type
                == progress_tracker.trackable_type,
                db_entities.TaskStatus.name == status.name,
            )
            result = await self.uow.session.execute(stmt)
            db_task_status = result.scalar_one_or_none()

            # If not, then create it
            if not db_task_status:
                db_task_status = db_entities.TaskStatus(
                    trackable_id=progress_tracker.trackable_id,
                    trackable_type=progress_tracker.trackable_type,
                    name=status.name,
                )
                self.uow.session.add(db_task_status)
                await self.uow.session.flush()

            # Now update the status
            db_task_status.state = status.state
            db_task_status.message = status.message
            db_task_status.error = str(status.error)
            db_task_status.total = status.total
            db_task_status.current = status.current

    async def _recursive_delete(self, progress_tracker: ProgressTracker) -> None:
        """Delete a task status and all children."""
        status = await progress_tracker.status()
        # First delete all children
        for child in progress_tracker.children:
            await self._recursive_delete(child)
        # Then delete the current task status
        stmt = delete(db_entities.TaskStatus).where(
            db_entities.TaskStatus.trackable_id == progress_tracker.trackable_id,
            db_entities.TaskStatus.trackable_type == progress_tracker.trackable_type,
            db_entities.TaskStatus.name == status.name,
        )
        await self.uow.session.execute(stmt)

    async def delete(self, progress_tracker: ProgressTracker) -> None:
        """Delete a task status and all children."""
        async with self.uow:
            await self._recursive_delete(progress_tracker)
