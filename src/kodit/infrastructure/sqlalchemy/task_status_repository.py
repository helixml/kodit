"""Task repository for the task queue."""

from collections.abc import Callable

import structlog
from sqlalchemy import select
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
                db_entities.TaskStatus.index_id == progress_tracker.index_id,
                db_entities.TaskStatus.name == status.name,
            )
            result = await self.uow.session.execute(stmt)
            db_task_status = result.scalar_one_or_none()

            # If not, then create it
            if not db_task_status:
                db_task_status = db_entities.TaskStatus(
                    index_id=progress_tracker.index_id,
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
