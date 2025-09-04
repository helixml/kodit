"""Task repository for the task queue."""

from collections.abc import Callable

import structlog
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.protocols import TaskStatusRepository
from kodit.domain.value_objects import Progress, ReportingState, TrackableType
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_task_status_repository(
    session_factory: Callable[[], AsyncSession],
) -> TaskStatusRepository:
    """Create an index repository."""
    uow = SqlAlchemyUnitOfWork(session_factory=session_factory)
    return SqlAlchemyTaskStatusRepository(uow)


class SqlAlchemyTaskStatusRepository(TaskStatusRepository):
    """Repository for persisting progress state only."""

    def __init__(self, uow: SqlAlchemyUnitOfWork) -> None:
        """Initialize the repository."""
        self.uow = uow
        self.log = structlog.get_logger(__name__)

    async def save_progress(self, progress: Progress) -> None:
        """Save a Progress state to database."""
        async with self.uow:
            stmt = select(db_entities.TaskStatus).where(
                db_entities.TaskStatus.trackable_id == progress.trackable_id,
                db_entities.TaskStatus.trackable_type == progress.trackable_type,
                db_entities.TaskStatus.name == progress.name,
            )
            result = await self.uow.session.execute(stmt)
            db_task_status = result.scalar_one_or_none()

            if not db_task_status:
                db_task_status = db_entities.TaskStatus(
                    trackable_id=progress.trackable_id,
                    trackable_type=progress.trackable_type,
                    name=progress.name,
                )
                self.uow.session.add(db_task_status)

            # Direct mapping from Progress to DB
            db_task_status.state = progress.state
            db_task_status.message = progress.message
            db_task_status.error = str(progress.error) if progress.error else ""
            db_task_status.total = progress.total
            db_task_status.current = progress.current
            # Parent relationship is handled by the caller

    async def load_progress_with_hierarchy(
        self, trackable_type: str, trackable_id: int
    ) -> list[tuple[int, Progress, int | None]]:
        """Load Progress states with IDs and parent IDs from database."""
        async with self.uow:
            stmt = select(db_entities.TaskStatus).where(
                db_entities.TaskStatus.trackable_id == trackable_id,
                db_entities.TaskStatus.trackable_type == trackable_type,
            )
            result = await self.uow.session.execute(stmt)
            db_statuses = list(result.scalars().all())

            # Return (db_id, Progress, parent_id) for hierarchy reconstruction
            return [
                (
                    db.id,  # Database ID
                    Progress(
                        name=db.name,
                        state=ReportingState(db.state),
                        message=db.message or "",
                        error=Exception(db.error) if db.error else None,
                        total=db.total,
                        current=db.current,
                        trackable_id=db.trackable_id,
                        trackable_type=TrackableType(db.trackable_type),
                    ),
                    db.parent,  # Parent ID from database
                )
                for db in db_statuses
            ]

    async def load_progress(
        self, trackable_type: str, trackable_id: int
    ) -> list[Progress]:
        """Load Progress states from database (without hierarchy info)."""
        progress_with_hierarchy = await self.load_progress_with_hierarchy(
            trackable_type, trackable_id
        )
        return [progress for _, progress, _ in progress_with_hierarchy]

    async def delete_progress(
        self, trackable_type: str, trackable_id: int, name: str
    ) -> None:
        """Delete a progress state."""
        async with self.uow:
            stmt = delete(db_entities.TaskStatus).where(
                db_entities.TaskStatus.trackable_id == trackable_id,
                db_entities.TaskStatus.trackable_type == trackable_type,
                db_entities.TaskStatus.name == name,
            )
            await self.uow.session.execute(stmt)
