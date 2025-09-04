"""Task repository for the task queue."""

from collections.abc import Callable

import structlog
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.reporting import ProgressTracker
from kodit.domain.protocols import TaskStatusRepository
from kodit.domain.value_objects import TrackableType
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

    async def delete(self, progress_tracker: ProgressTracker) -> None:
        """Delete a task status and all children."""
        status = await progress_tracker.status()
        async with self.uow:
            stmt = delete(db_entities.TaskStatus).where(
                db_entities.TaskStatus.trackable_id == progress_tracker.trackable_id,
                db_entities.TaskStatus.trackable_type
                == progress_tracker.trackable_type,
                db_entities.TaskStatus.name == status.name,
            )
            await self.uow.session.execute(stmt)

    async def _to_progress_tracker(
        self, db_task_status: db_entities.TaskStatus
    ) -> ProgressTracker:
        """Convert a database task status to a progress tracker."""
        p = ProgressTracker(
            name=db_task_status.name,
        )
        await p.set_tracking_info(
            db_task_status.trackable_id, TrackableType(db_task_status.trackable_type)
        )
        await p.set_total(db_task_status.total)
        await p.set_current(db_task_status.current)
        return p

    async def _to_list_of_parents(
        self, db_task_statuses: list[db_entities.TaskStatus]
    ) -> dict[int, ProgressTracker]:
        """Convert a list of database task statuses to a map of progress trackers."""
        return {
            db_task_status.parent: await self._to_progress_tracker(db_task_status)
            for db_task_status in db_task_statuses
        }

    async def _to_progress_tracker_list(
        self, db_task_statuses: list[db_entities.TaskStatus]
    ) -> list[ProgressTracker]:
        """Convert a list of database task statuses to a list of progress trackers."""
        parents = await self._to_list_of_parents(db_task_statuses)
        final = []
        for db_status in db_task_statuses:
            p = await self._to_progress_tracker(db_status)
            p.parent = parents[db_status.parent]
            final.append(p)
        return final

    async def find(
        self, trackable_type: str, trackable_id: int
    ) -> list[ProgressTracker]:
        """Find a task status by trackable type and ID."""
        async with self.uow:
            stmt = select(db_entities.TaskStatus).where(
                db_entities.TaskStatus.trackable_id == trackable_id,
                db_entities.TaskStatus.trackable_type == trackable_type,
            )
            result = await self.uow.session.execute(stmt)
            db_task_statuses = list(result.scalars().all())
            return await self._to_progress_tracker_list(db_task_statuses)
