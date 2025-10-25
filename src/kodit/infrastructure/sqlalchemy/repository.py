"""Abstract base classes for repositories."""

from abc import ABC, abstractmethod
from collections.abc import Callable, Generator
from typing import Any, Generic, TypeVar

from sqlalchemy import inspect, select
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

    @abstractmethod
    def _get_id(self, entity: DomainEntityType) -> Any:
        """Extract ID from domain entity."""

    @property
    @abstractmethod
    def db_entity_type(self) -> type[DatabaseEntityType]:
        """The SQLAlchemy model type."""

    @abstractmethod
    def to_domain(self, db_entity: DatabaseEntityType) -> DomainEntityType:
        """Map database entity to domain entity."""

    @abstractmethod
    def to_db(self, domain_entity: DomainEntityType) -> DatabaseEntityType:
        """Map domain entity to database entity."""

    def _update_db_entity(
        self, existing: DatabaseEntityType, new: DatabaseEntityType
    ) -> None:
        """Update existing database entity with values from new entity."""
        mapper = inspect(type(existing))
        if mapper is None:
            return
        for column in mapper.columns:
            if not column.primary_key:
                setattr(existing, column.key, getattr(new, column.key))

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

    async def save(self, entity: DomainEntityType) -> None:
        """Save entity (create new or update existing)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            entity_id = self._get_id(entity)
            existing_db_entity = await session.get(self.db_entity_type, entity_id)

            if existing_db_entity:
                # Update existing entity
                new_db_entity = self.to_db(entity)
                self._update_db_entity(existing_db_entity, new_db_entity)
            else:
                # Create new entity
                db_entity = self.to_db(entity)
                session.add(db_entity)

            await session.flush()

    async def save_bulk(self, entities: list[DomainEntityType]) -> None:
        """Save multiple entities in bulk (create new or update existing)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            for chunk in self._chunked(entities):
                # Get IDs for all entities in chunk
                entity_ids = [self._get_id(entity) for entity in chunk]

                # Fetch all existing entities in one query
                existing_entities = {}
                for entity_id in entity_ids:
                    existing = await session.get(self.db_entity_type, entity_id)
                    if existing:
                        existing_entities[entity_id] = existing

                # Process each entity
                new_entities = []
                for entity in chunk:
                    entity_id = self._get_id(entity)
                    new_db_entity = self.to_db(entity)

                    if entity_id in existing_entities:
                        # Update existing entity
                        existing = existing_entities[entity_id]
                        self._update_db_entity(existing, new_db_entity)
                    else:
                        # Collect new entities to add
                        new_entities.append(new_db_entity)

                # Add all new entities at once
                if new_entities:
                    session.add_all(new_entities)

                await session.flush()

    async def exists(self, entity_id: Any) -> bool:
        """Check if entity exists by primary key."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_entity = await session.get(self.db_entity_type, entity_id)
            return db_entity is not None

    async def delete(self, entity: DomainEntityType) -> None:
        """Remove entity."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            db_entity = await session.get(self.db_entity_type, self._get_id(entity))
            if db_entity:
                await session.delete(db_entity)

    async def delete_bulk(self, entities: list[DomainEntityType]) -> None:
        """Remove multiple entities in bulk using chunking."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            for chunk in self._chunked(entities):
                entity_ids = [self._get_id(entity) for entity in chunk]
                for entity_id in entity_ids:
                    db_entity = await session.get(self.db_entity_type, entity_id)
                    if db_entity:
                        await session.delete(db_entity)
                await session.flush()

    def _chunked(
        self, items: list[DomainEntityType], chunk_size: int | None = None
    ) -> Generator[list[DomainEntityType], None, None]:
        """Yield chunks of items."""
        chunk_size = chunk_size or self._chunk_size
        for i in range(0, len(items), chunk_size):
            yield items[i : i + chunk_size]
