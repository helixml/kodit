"""Abstract base classes for repositories."""

from abc import ABC, abstractmethod
from collections.abc import Callable, Generator
from typing import Any, Generic, TypeVar

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.infrastructure.sqlalchemy.query import Query
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

DomainEntityType = TypeVar("DomainEntityType")
DatabaseEntityType = TypeVar("DatabaseEntityType")


class SqlAlchemyRepository(ABC, Generic[DomainEntityType, DatabaseEntityType]):
    """Base repository with common SQLAlchemy patterns."""

    def __init__(self, session_factory: Callable[[], AsyncSession]) -> None:
        """Initialize the repository."""
        self.session_factory = session_factory
        self._chunk_size = 1000

    @property
    @abstractmethod
    def db_entity_type(self) -> type[DatabaseEntityType]:
        """The SQLAlchemy model type."""

    @abstractmethod
    def to_domain(self, db_entity: DatabaseEntityType) -> DomainEntityType:
        """Map database entity to domain entity."""

    @abstractmethod
    def to_db(
        self, domain_entity: DomainEntityType, **kwargs: Any
    ) -> DatabaseEntityType:
        """Map domain entity to database entity."""

    async def get(self, entity_id: Any) -> DomainEntityType:
        """Get entity by primary key."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_entity = await session.get(self.db_entity_type, entity_id)
            if not db_entity:
                raise ValueError(f"Entity with id {entity_id} not found")
            return self.to_domain(db_entity)

    async def find(self, query: Query) -> list[DomainEntityType]:
        """Find all entities matching query."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(self.db_entity_type)
            stmt = query.apply(stmt, self.db_entity_type)
            db_entities = (await session.scalars(stmt)).all()
            return [self.to_domain(db) for db in db_entities]

    async def add(self, entity: DomainEntityType, **kwargs: Any) -> DomainEntityType:
        """Add new entity."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_entity = self.to_db(entity, **kwargs)
            session.add(db_entity)
            await session.flush()
            return entity

    async def exists(self, entity_id: Any) -> bool:
        """Check if entity exists by primary key."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_entity = await session.get(self.db_entity_type, entity_id)
            return db_entity is not None

    async def remove(self, entity: DomainEntityType) -> None:
        """Remove entity."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_entity = await session.get(self.db_entity_type, self._get_id(entity))
            if db_entity:
                await session.delete(db_entity)

    @abstractmethod
    def _get_id(self, entity: DomainEntityType) -> Any:
        """Extract ID from domain entity."""

    def _chunked(
        self, items: list[DomainEntityType], chunk_size: int | None = None
    ) -> Generator[list[DomainEntityType], None, None]:
        """Yield chunks of items."""
        chunk_size = chunk_size or self._chunk_size
        for i in range(0, len(items), chunk_size):
            yield items[i : i + chunk_size]
