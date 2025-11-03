"""Task repository for the task queue."""

from collections.abc import Callable
from typing import Any

import structlog
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

import kodit.domain.entities as domain_entities
from kodit.domain.protocols import TaskRepository
from kodit.infrastructure.mappers.task_mapper import TaskMapper
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


def create_task_repository(
    session_factory: Callable[[], AsyncSession],
) -> TaskRepository:
    """Create an index repository."""
    return SqlAlchemyTaskRepository(session_factory=session_factory)


# TODO(Phil): Stop using dedup_key as the primary key. Add some DDD to this instead.


class SqlAlchemyTaskRepository(
    SqlAlchemyRepository[domain_entities.Task, db_entities.Task], TaskRepository
):
    """Repository for task persistence using the existing Task entity."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        super().__init__(session_factory)
        self.log = structlog.get_logger(__name__)

    @property
    def db_entity_type(self) -> type[db_entities.Task]:
        """The SQLAlchemy model type."""
        return db_entities.Task

    def _get_id(self, entity: domain_entities.Task) -> Any:
        """Extract ID from domain entity."""
        return entity.id

    @staticmethod
    def to_domain(db_entity: db_entities.Task) -> domain_entities.Task:
        """Map database entity to domain entity."""
        return TaskMapper.to_domain_task(db_entity)

    @staticmethod
    def to_db(domain_entity: domain_entities.Task) -> db_entities.Task:
        """Map domain entity to database entity."""
        return TaskMapper.from_domain_task(domain_entity)

    async def get(self, entity_id: Any) -> domain_entities.Task:
        """Get entity by dedup_key."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.Task).where(
                db_entities.Task.dedup_key == entity_id
            )
            result = await session.execute(stmt)
            db_entity = result.scalar_one_or_none()
            if not db_entity:
                raise ValueError(f"Entity with id {entity_id} not found")
            return self.to_domain(db_entity)

    async def exists(self, entity_id: Any) -> bool:
        """Check if entity exists by dedup_key."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.Task).where(
                db_entities.Task.dedup_key == entity_id
            )
            result = await session.execute(stmt)
            return result.scalar_one_or_none() is not None

    async def save(self, entity: domain_entities.Task) -> domain_entities.Task:
        """Save entity (create new or update existing)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            # Query by dedup_key instead of primary key
            stmt = select(db_entities.Task).where(
                db_entities.Task.dedup_key == entity.id
            )
            result = await session.execute(stmt)
            existing_db_entity = result.scalar_one_or_none()

            if existing_db_entity:
                # Update existing entity
                new_db_entity = self.to_db(entity)
                self._update_db_entity(existing_db_entity, new_db_entity)
                db_entity = existing_db_entity
            else:
                # Create new entity
                db_entity = self.to_db(entity)
                session.add(db_entity)

            await session.flush()
            return self.to_domain(db_entity)

    async def delete(self, entity: domain_entities.Task) -> None:
        """Remove entity by dedup_key."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(db_entities.Task).where(
                db_entities.Task.dedup_key == entity.id
            )
            result = await session.execute(stmt)
            db_entity = result.scalar_one_or_none()
            if db_entity:
                await session.delete(db_entity)

    async def next(self) -> domain_entities.Task | None:
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
