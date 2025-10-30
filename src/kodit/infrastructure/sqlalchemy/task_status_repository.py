"""Task repository for the task queue."""

from collections.abc import Callable
from typing import Any, override

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain import entities as domain_entities
from kodit.domain.protocols import TaskStatusRepository
from kodit.infrastructure.mappers.task_status_mapper import TaskStatusMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_task_status_repository(
    session_factory: Callable[[], AsyncSession],
) -> TaskStatusRepository:
    """Create an index repository."""
    return SqlAlchemyTaskStatusRepository(session_factory=session_factory)


class SqlAlchemyTaskStatusRepository(
    SqlAlchemyRepository[domain_entities.TaskStatus, db_entities.TaskStatus],
    TaskStatusRepository,
):
    """Repository for persisting TaskStatus entities."""

    @property
    def db_entity_type(self) -> type[db_entities.TaskStatus]:
        """The SQLAlchemy model type."""
        return db_entities.TaskStatus

    def _get_id(self, entity: domain_entities.TaskStatus) -> Any:
        """Extract ID from domain entity."""
        return entity.id

    @staticmethod
    def to_domain(
        db_entity: db_entities.TaskStatus,
    ) -> domain_entities.TaskStatus:
        """Map database entity to domain entity."""
        return TaskStatusMapper.to_domain_task_status(db_entity)

    @staticmethod
    def to_db(
        domain_entity: domain_entities.TaskStatus,
    ) -> db_entities.TaskStatus:
        """Map domain entity to database entity."""
        return TaskStatusMapper.from_domain_task_status(domain_entity)

    @override
    async def save(
        self, entity: domain_entities.TaskStatus
    ) -> domain_entities.TaskStatus:
        """Save a TaskStatus to database."""
        # Recursively convert parents to a list of domain entities, parents first
        parents: list[domain_entities.TaskStatus] = []
        current = entity
        while current.parent is not None:
            parents.insert(0, current.parent)
            current = current.parent

        # Add current entity to the end of the list
        parents.append(entity)

        await self.save_bulk(parents)
        return entity

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
            return TaskStatusMapper.to_domain_task_status_with_hierarchy(db_statuses)
