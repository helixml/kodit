"""Abstract base classes for repositories."""

from abc import ABC, abstractmethod
from collections.abc import Callable, Generator
from typing import Any, Generic, TypeVar

from sqlalchemy import func, inspect, select
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

    @staticmethod
    @abstractmethod
    def to_domain(db_entity: DatabaseEntityType) -> DomainEntityType:
        """Map database entity to domain entity."""

    @staticmethod
    @abstractmethod
    def to_db(domain_entity: DomainEntityType) -> DatabaseEntityType:
        """Map domain entity to database entity."""

    def _update_db_entity(
        self, existing: DatabaseEntityType, new: DatabaseEntityType
    ) -> None:
        """Update existing database entity with values from new entity."""
        mapper = inspect(type(existing))
        if mapper is None:
            return
        # Skip auto-managed columns
        skip_columns = {"created_at", "updated_at", "id"}
        for column in mapper.columns:
            if not column.primary_key and column.key not in skip_columns:
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
        from kodit.infrastructure.sqlalchemy.query import QueryBuilder

        # Check if we need to chunk IN queries
        if isinstance(query, QueryBuilder):
            large_in_filters = query.get_large_in_filters(self._chunk_size)

            if large_in_filters:
                # We need to chunk the query
                return await self._find_with_chunked_in(query, large_in_filters)

        # Normal case: no chunking needed
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(self.db_entity_type)
            stmt = query.apply(stmt, self.db_entity_type)
            db_entities = (await session.scalars(stmt)).all()
            return [self.to_domain(db) for db in db_entities]

    async def _find_with_chunked_in(
        self, query: Query, large_in_filters: list[Any]
    ) -> list[DomainEntityType]:
        """Execute find query with chunked IN filters."""
        from kodit.infrastructure.sqlalchemy.query import (
            FilterCriteria,
            QueryBuilder,
        )

        # For simplicity, we'll only handle the case of a single large IN filter
        if len(large_in_filters) > 1:
            raise ValueError("Multiple large IN filters not supported")

        # Type narrowing for mypy
        if not isinstance(query, QueryBuilder):
            raise TypeError("Query must be a QueryBuilder for chunking")

        large_filter = large_in_filters[0]
        all_results = []

        # Chunk the IN filter values
        for i in range(0, len(large_filter.value), self._chunk_size):
            chunk_values = large_filter.value[i : i + self._chunk_size]

            # Create a new query with the chunked IN filter
            chunked_filter = FilterCriteria(
                field=large_filter.field,
                operator=large_filter.operator,
                value=chunk_values,
            )
            chunked_query = query.with_replaced_filter(large_filter, chunked_filter)

            # Execute the chunked query
            async with SqlAlchemyUnitOfWork(self.session_factory) as session:
                stmt = select(self.db_entity_type)
                stmt = chunked_query.apply(stmt, self.db_entity_type)
                db_entities = (await session.scalars(stmt)).all()
                all_results.extend([self.to_domain(db) for db in db_entities])

        return all_results

    async def count(self, query: Query) -> int:
        """Count the number of entities matching query."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(self.db_entity_type).with_only_columns(func.count())
            # For count queries, only apply filters, not sorting or pagination
            from kodit.infrastructure.sqlalchemy.query import QueryBuilder

            if isinstance(query, QueryBuilder):
                # Apply only filters, skip sorting and pagination for count queries
                stmt = query.apply_filters_only(stmt, self.db_entity_type)
            else:
                stmt = query.apply(stmt, self.db_entity_type)
            result = await session.scalar(stmt)
            return result or 0

    async def save(self, entity: DomainEntityType) -> DomainEntityType:
        """Save entity (create new or update existing)."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            entity_id = self._get_id(entity)
            # Skip session.get if entity_id is None (new entity not yet persisted)
            existing_db_entity = (
                await session.get(self.db_entity_type, entity_id)
                if entity_id is not None
                else None
            )

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

    async def save_bulk(
        self,
        entities: list[DomainEntityType],
        *,
        skip_existence_check: bool = False,
    ) -> list[DomainEntityType]:
        """Save multiple entities in bulk (create new or update existing).

        Args:
            entities: List of domain entities to save
            skip_existence_check: If True, skip checking if entities exist
                (faster for new entities)

        Returns:
            List of saved domain entities

        """
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            all_saved_db_entities = []

            for chunk in self._chunked_domain(entities):
                if skip_existence_check:
                    # Fast path: assume all entities are new
                    new_entities = [self.to_db(entity) for entity in chunk]
                    session.add_all(new_entities)
                    await session.flush()
                    all_saved_db_entities.extend(new_entities)
                else:
                    # Fetch existing entities in a single bulk query
                    existing_entities = await self._fetch_existing_entities_bulk(
                        session, chunk
                    )

                    # Process each entity
                    new_entities = []
                    chunk_db_entities = []
                    for entity in chunk:
                        entity_id = self._get_id(entity)
                        new_db_entity = self.to_db(entity)

                        if entity_id in existing_entities:
                            # Update existing entity
                            existing = existing_entities[entity_id]
                            self._update_db_entity(existing, new_db_entity)
                            chunk_db_entities.append(existing)
                        else:
                            # Collect new entities to add
                            new_entities.append(new_db_entity)
                            chunk_db_entities.append(new_db_entity)

                    # Add all new entities at once
                    if new_entities:
                        session.add_all(new_entities)

                    await session.flush()
                    all_saved_db_entities.extend(chunk_db_entities)

            return [self.to_domain(db) for db in all_saved_db_entities]

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

    async def delete_by_query(self, query: Query) -> None:
        """Remove entities by query."""
        async with SqlAlchemyUnitOfWork(self.session_factory) as session:
            stmt = select(self.db_entity_type)
            stmt = query.apply(stmt, self.db_entity_type)
            db_entities = list((await session.scalars(stmt)).all())
            if not db_entities:
                return
            for chunk in self._chunked_db(db_entities):
                for db_entity in chunk:
                    await session.delete(db_entity)
            await session.flush()

    def _chunked_domain(
        self,
        items: list[DomainEntityType],
        chunk_size: int | None = None,
    ) -> Generator[list[DomainEntityType], None, None]:
        """Yield chunks of items."""
        chunk_size = chunk_size or self._chunk_size
        for i in range(0, len(items), chunk_size):
            yield items[i : i + chunk_size]

    def _chunked_db(
        self,
        items: list[DatabaseEntityType],
        chunk_size: int | None = None,
    ) -> Generator[list[DatabaseEntityType], None, None]:
        """Yield chunks of items."""
        chunk_size = chunk_size or self._chunk_size
        for i in range(0, len(items), chunk_size):
            yield items[i : i + chunk_size]

    async def _fetch_existing_entities_bulk(
        self, session: AsyncSession, entities: list[DomainEntityType]
    ) -> dict[Any, DatabaseEntityType]:
        """Fetch existing entities in a single bulk query.

        Handles both simple primary keys and composite primary keys.

        Args:
            session: SQLAlchemy session
            entities: List of domain entities to check

        Returns:
            Dictionary mapping entity_id to database entity

        """
        from sqlalchemy import tuple_
        from sqlalchemy.inspection import inspect

        # Get entity IDs, filtering out None
        entity_ids = [self._get_id(entity) for entity in entities]
        entity_ids = [eid for eid in entity_ids if eid is not None]

        if not entity_ids:
            return {}

        # Get primary key columns
        mapper = inspect(self.db_entity_type)
        if mapper is None:
            return {}
        pk_columns = [col.key for col in mapper.primary_key]

        if len(pk_columns) == 1:
            # Simple primary key - use IN clause
            pk_col = getattr(self.db_entity_type, pk_columns[0])
            stmt = select(self.db_entity_type).where(pk_col.in_(entity_ids))
        else:
            # Composite primary key - use tuple IN clause
            pk_tuple = tuple_(
                *[getattr(self.db_entity_type, col) for col in pk_columns]
            )
            stmt = select(self.db_entity_type).where(pk_tuple.in_(entity_ids))

        # Execute query and build result dict
        result = await session.scalars(stmt)
        existing = result.all()

        # Map entities by their ID for quick lookup
        return {self._get_id_from_db(db_entity): db_entity for db_entity in existing}

    def _get_id_from_db(self, db_entity: DatabaseEntityType) -> Any:
        """Extract ID from database entity.

        For simple primary keys, returns the value directly.
        For composite keys, returns a tuple of values.
        """
        from sqlalchemy.inspection import inspect

        mapper = inspect(self.db_entity_type)
        if mapper is None:
            return None
        pk_columns = [col.key for col in mapper.primary_key]

        if len(pk_columns) == 1:
            return getattr(db_entity, pk_columns[0])
        return tuple(getattr(db_entity, col) for col in pk_columns)
