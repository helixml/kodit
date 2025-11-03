"""Tests for SqlAlchemyRepository base class."""

from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

import pytest
from sqlalchemy import DateTime, Integer, String
from sqlalchemy.ext.asyncio import AsyncEngine, AsyncSession
from sqlalchemy.orm import Mapped, mapped_column

from kodit.infrastructure.api.v1.query_params import PaginationParams
from kodit.infrastructure.sqlalchemy.entities import Base
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository

# Test entities for repository testing


@dataclass
class MockEntity:
    """Simple domain entity for testing."""

    id: int
    name: str
    value: int


class MockDbEntity(Base):
    """Database entity for testing."""

    __tablename__ = "test_entities"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    name: Mapped[str] = mapped_column(String(100), nullable=False)
    value: Mapped[int] = mapped_column(Integer, nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime, nullable=False, default=lambda: datetime.now(UTC)
    )


class MockRepository(SqlAlchemyRepository[MockEntity, MockDbEntity]):
    """Concrete repository implementation for testing."""

    @property
    def db_entity_type(self) -> type[MockDbEntity]:
        """Return the database entity type."""
        return MockDbEntity

    def _get_id(self, entity: MockEntity) -> Any:
        """Extract ID from domain entity."""
        return entity.id

    @staticmethod
    def to_domain(db_entity: MockDbEntity) -> MockEntity:
        """Map database entity to domain entity."""
        return MockEntity(
            id=db_entity.id,
            name=db_entity.name,
            value=db_entity.value,
        )

    @staticmethod
    def to_db(domain_entity: MockEntity) -> MockDbEntity:
        """Map domain entity to database entity."""
        return MockDbEntity(
            id=domain_entity.id,
            name=domain_entity.name,
            value=domain_entity.value,
        )


@pytest.fixture
async def create_test_table(engine: AsyncEngine) -> None:
    """Create the test table in the database."""
    async with engine.begin() as conn:
        # Drop and recreate to ensure fresh schema
        await conn.run_sync(MockDbEntity.metadata.drop_all)
        await conn.run_sync(MockDbEntity.metadata.create_all)


@pytest.fixture
def repository(
    session_factory: Callable[[], AsyncSession],
    create_test_table: None,  # noqa: ARG001
) -> MockRepository:
    """Create a repository with a session factory."""
    return MockRepository(session_factory)


class TestSave:
    """Tests for the save method."""

    async def test_saves_new_entity(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies repository can persist a new entity."""
        entity = MockEntity(id=1, name="test", value=42)

        await repository.save(entity)

        # Verify it was actually saved
        retrieved_entity = await repository.get(1)
        assert retrieved_entity.id == 1
        assert retrieved_entity.name == "test"
        assert retrieved_entity.value == 42

    async def test_updates_existing_entity(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies repository updates an existing entity instead of duplicates."""
        # Create initial entity
        entity = MockEntity(id=1, name="original", value=10)
        await repository.save(entity)

        # Verify initial state
        retrieved = await repository.get(1)
        assert retrieved.name == "original"
        assert retrieved.value == 10

        # Update the same entity
        updated_entity = MockEntity(id=1, name="updated", value=20)
        await repository.save(updated_entity)

        # Verify the entity was updated, not duplicated
        final = await repository.get(1)
        assert final.id == 1
        assert final.name == "updated"
        assert final.value == 20

        # Verify no duplicate was created
        query = QueryBuilder()
        all_entities = await repository.find(query)
        assert len(all_entities) == 1

    async def test_updates_only_non_primary_key_fields(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies that updates only affect non-primary key columns."""
        # Create initial entity
        entity = MockEntity(id=1, name="test", value=100)
        await repository.save(entity)

        # Update with same ID but different values
        updated = MockEntity(id=1, name="changed", value=200)
        await repository.save(updated)

        # Verify the update
        result = await repository.get(1)
        assert result.id == 1
        assert result.name == "changed"
        assert result.value == 200


class TestSaveBulk:
    """Tests for the save_bulk method."""

    async def test_saves_multiple_new_entities(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies repository can persist multiple new entities at once."""
        entities = [
            MockEntity(id=1, name="first", value=10),
            MockEntity(id=2, name="second", value=20),
            MockEntity(id=3, name="third", value=30),
        ]

        await repository.save_bulk(entities)

        # Verify all were saved
        query = QueryBuilder()
        all_entities = await repository.find(query)
        assert len(all_entities) == 3

        # Verify individual entities
        entity1 = await repository.get(1)
        assert entity1.name == "first"
        entity2 = await repository.get(2)
        assert entity2.name == "second"
        entity3 = await repository.get(3)
        assert entity3.name == "third"

    async def test_updates_existing_entities_in_bulk(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies repository updates existing entities in bulk."""
        # Create initial entities
        initial = [
            MockEntity(id=1, name="first", value=10),
            MockEntity(id=2, name="second", value=20),
        ]
        await repository.save_bulk(initial)

        # Update the same entities
        updated = [
            MockEntity(id=1, name="updated_first", value=100),
            MockEntity(id=2, name="updated_second", value=200),
        ]
        await repository.save_bulk(updated)

        # Verify updates worked and no duplicates were created
        query = QueryBuilder()
        all_entities = await repository.find(query)
        assert len(all_entities) == 2

        entity1 = await repository.get(1)
        assert entity1.name == "updated_first"
        assert entity1.value == 100

        entity2 = await repository.get(2)
        assert entity2.name == "updated_second"
        assert entity2.value == 200

    async def test_handles_mixed_new_and_existing_entities(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies repository can handle both new and existing entities in one call."""
        # Create one existing entity
        existing = MockEntity(id=1, name="existing", value=10)
        await repository.save(existing)

        # Mix of update and new entities
        mixed = [
            MockEntity(id=1, name="updated", value=100),  # Update existing
            MockEntity(id=2, name="new_one", value=20),  # New
            MockEntity(id=3, name="new_two", value=30),  # New
        ]
        await repository.save_bulk(mixed)

        # Verify all entities exist with correct values
        query = QueryBuilder()
        all_entities = await repository.find(query)
        assert len(all_entities) == 3

        entity1 = await repository.get(1)
        assert entity1.name == "updated"
        assert entity1.value == 100

        entity2 = await repository.get(2)
        assert entity2.name == "new_one"

        entity3 = await repository.get(3)
        assert entity3.name == "new_two"

    async def test_handles_empty_list(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies repository handles empty entity lists gracefully."""
        await repository.save_bulk([])

        query = QueryBuilder()
        all_entities = await repository.find(query)
        assert len(all_entities) == 0


class TestDelete:
    """Tests for the delete method."""

    async def test_deletes_existing_entity(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies repository can delete an existing entity."""
        # Create entity
        entity = MockEntity(id=1, name="to_delete", value=42)
        await repository.save(entity)

        # Verify it exists
        assert await repository.exists(1)

        # Delete it
        await repository.delete(entity)

        # Verify it's gone
        assert not await repository.exists(1)

    async def test_deletes_nonexistent_entity_gracefully(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies deletion of non-existent entity doesn't cause error."""
        entity = MockEntity(id=999, name="nonexistent", value=0)

        # Should not raise an error
        await repository.delete(entity)

        # Verify it still doesn't exist
        assert not await repository.exists(999)

    async def test_deletes_correct_entity_when_multiple_exist(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies only the specified entity is deleted."""
        # Create multiple entities
        entities = [
            MockEntity(id=1, name="keep", value=10),
            MockEntity(id=2, name="delete_me", value=20),
            MockEntity(id=3, name="keep_too", value=30),
        ]
        await repository.save_bulk(entities)

        # Delete only the middle one
        await repository.delete(entities[1])

        # Verify correct entity was deleted
        assert await repository.exists(1)
        assert not await repository.exists(2)
        assert await repository.exists(3)

        # Verify count
        query = QueryBuilder()
        remaining = await repository.find(query)
        assert len(remaining) == 2

    async def test_delete_by_query_removes_matching_entities(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies delete_by_query removes all entities matching the query."""
        await repository.save_bulk(
            [
                MockEntity(id=1, name="keep", value=10),
                MockEntity(id=2, name="remove", value=20),
                MockEntity(id=3, name="remove", value=30),
            ]
        )

        await repository.delete_by_query(
            QueryBuilder().filter("name", FilterOperator.EQ, "remove")
        )

        assert len(await repository.find(QueryBuilder())) == 1
        assert await repository.exists(1)


class TestFind:
    """Tests for the find method."""

    async def test_finds_all_entities_with_empty_query(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies find returns all entities when given empty query."""
        # Create entities
        entities = [
            MockEntity(id=1, name="first", value=10),
            MockEntity(id=2, name="second", value=20),
            MockEntity(id=3, name="third", value=30),
        ]
        await repository.save_bulk(entities)

        # Find all
        query = QueryBuilder()
        found = await repository.find(query)

        assert len(found) == 3
        ids = {e.id for e in found}
        assert ids == {1, 2, 3}

    async def test_finds_entities_with_filters(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies find applies query filters correctly."""
        # Create entities with different values
        entities = [
            MockEntity(id=1, name="match", value=100),
            MockEntity(id=2, name="no_match", value=50),
            MockEntity(id=3, name="match", value=100),
        ]
        await repository.save_bulk(entities)

        # Create query with filter
        query = QueryBuilder().filter("value", FilterOperator.EQ, 100)
        found = await repository.find(query)

        assert len(found) == 2
        for entity in found:
            assert entity.value == 100

    async def test_finds_entities_with_limit(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies find respects limit in query."""
        # Create many entities
        entities = [MockEntity(id=i, name=f"entity_{i}", value=i) for i in range(1, 11)]
        await repository.save_bulk(entities)

        # Query with limit
        query = QueryBuilder().paginate(PaginationParams(page=1, page_size=5))
        found = await repository.find(query)

        assert len(found) == 5

    async def test_returns_empty_list_when_no_matches(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies find returns empty list when no entities match."""
        # Create entities
        entities = [MockEntity(id=1, name="test", value=10)]
        await repository.save_bulk(entities)

        # Query that won't match
        query = QueryBuilder().filter("value", FilterOperator.EQ, 999)
        found = await repository.find(query)

        assert found == []

    async def test_returns_empty_list_when_table_empty(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies find returns empty list when table is empty."""
        query = QueryBuilder()
        found = await repository.find(query)

        assert found == []

    async def test_chunks_large_in_queries(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies find chunks large IN queries to avoid database parameter limits."""
        # Create more entities than the chunk size (1000)
        num_entities = 2500
        entities = [
            MockEntity(id=i, name=f"entity_{i}", value=i)
            for i in range(1, num_entities + 1)
        ]
        await repository.save_bulk(entities)

        # Create a query with a large IN clause
        ids_to_find = list(range(1, num_entities + 1))
        query = QueryBuilder().filter("id", FilterOperator.IN, ids_to_find)
        found = await repository.find(query)

        # Should find all entities despite exceeding chunk size
        assert len(found) == num_entities
        found_ids = {e.id for e in found}
        assert found_ids == set(ids_to_find)

    async def test_chunks_preserves_other_filters(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies that chunking preserves other filters in the query."""
        # Create entities with different values
        entities = [
            MockEntity(id=i, name=f"entity_{i}", value=100 if i % 2 == 0 else 50)
            for i in range(1, 2001)
        ]
        await repository.save_bulk(entities)

        # Query with both IN and EQ filters
        ids_to_find = list(range(1, 2001))
        query = (
            QueryBuilder()
            .filter("id", FilterOperator.IN, ids_to_find)
            .filter("value", FilterOperator.EQ, 100)
        )
        found = await repository.find(query)

        # Should only find entities with value=100
        assert len(found) == 1000
        for entity in found:
            assert entity.value == 100
            assert entity.id % 2 == 0


class TestExists:
    """Tests for the exists method."""

    async def test_returns_true_for_existing_entity(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies exists returns True for existing entity."""
        entity = MockEntity(id=1, name="test", value=42)
        await repository.save(entity)

        assert await repository.exists(1)

    async def test_returns_false_for_nonexistent_entity(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies exists returns False for non-existent entity."""
        assert not await repository.exists(999)

    async def test_returns_false_after_deletion(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies exists returns False after entity is deleted."""
        entity = MockEntity(id=1, name="test", value=42)
        await repository.save(entity)

        assert await repository.exists(1)

        await repository.delete(entity)

        assert not await repository.exists(1)


class TestCount:
    """Tests for the count method."""

    async def test_returns_correct_count(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies count returns the correct number of entities."""
        entities = [MockEntity(id=1, name="test", value=42)]
        await repository.save_bulk(entities)
        assert await repository.count(QueryBuilder()) == 1

    async def test_with_query(
        self,
        repository: MockRepository,
    ) -> None:
        """Verifies count returns the correct number of entities with a query."""
        entities = [
            MockEntity(id=1, name="test", value=42),
            MockEntity(id=2, name="test2", value=31),
        ]
        await repository.save_bulk(entities)
        assert (
            await repository.count(
                QueryBuilder().filter("value", FilterOperator.EQ, 42)
            )
            == 1
        )
