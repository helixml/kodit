"""Tests for SqlAlchemyRepository base class."""

from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

import pytest
from sqlalchemy import Integer, String
from sqlalchemy.ext.asyncio import AsyncEngine, AsyncSession
from sqlalchemy.orm import Mapped, mapped_column

from kodit.infrastructure.sqlalchemy.entities import Base
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemyRepository

# Test entities for repository testing


@dataclass
class TestEntity:
    """Simple domain entity for testing."""

    id: int
    name: str
    value: int


class TestDbEntity(Base):
    """Database entity for testing."""

    __tablename__ = "test_entities"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    name: Mapped[str] = mapped_column(String(100), nullable=False)
    value: Mapped[int] = mapped_column(Integer, nullable=False)


class TestRepository(SqlAlchemyRepository[TestEntity, TestDbEntity]):
    """Concrete repository implementation for testing."""

    @property
    def db_entity_type(self) -> type[TestDbEntity]:
        """Return the database entity type."""
        return TestDbEntity

    def _get_id(self, entity: TestEntity) -> Any:
        """Extract ID from domain entity."""
        return entity.id

    def to_domain(self, db_entity: TestDbEntity) -> TestEntity:
        """Map database entity to domain entity."""
        return TestEntity(
            id=db_entity.id,
            name=db_entity.name,
            value=db_entity.value,
        )

    def to_db(self, domain_entity: TestEntity) -> TestDbEntity:
        """Map domain entity to database entity."""
        return TestDbEntity(
            id=domain_entity.id,
            name=domain_entity.name,
            value=domain_entity.value,
        )


@pytest.fixture
async def create_test_table(engine: AsyncEngine) -> None:
    """Create the test table in the database."""
    async with engine.begin() as conn:
        await conn.run_sync(TestDbEntity.metadata.create_all)


@pytest.fixture
def repository(
    session_factory: Callable[[], AsyncSession],
    create_test_table: None,  # noqa: ARG001
) -> TestRepository:
    """Create a repository with a session factory."""
    return TestRepository(session_factory)


class TestSave:
    """Tests for the save method."""

    async def test_saves_new_entity(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies repository can persist a new entity."""
        entity = TestEntity(id=1, name="test", value=42)

        saved_entity = await repository.save(entity)

        assert saved_entity.id == entity.id
        assert saved_entity.name == entity.name
        assert saved_entity.value == entity.value

        # Verify it was actually saved
        retrieved_entity = await repository.get(1)
        assert retrieved_entity.id == 1
        assert retrieved_entity.name == "test"
        assert retrieved_entity.value == 42

    async def test_updates_existing_entity(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies repository updates an existing entity instead of duplicates."""
        # Create initial entity
        entity = TestEntity(id=1, name="original", value=10)
        await repository.save(entity)

        # Verify initial state
        retrieved = await repository.get(1)
        assert retrieved.name == "original"
        assert retrieved.value == 10

        # Update the same entity
        updated_entity = TestEntity(id=1, name="updated", value=20)
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
        repository: TestRepository,
    ) -> None:
        """Verifies that updates only affect non-primary key columns."""
        # Create initial entity
        entity = TestEntity(id=1, name="test", value=100)
        await repository.save(entity)

        # Update with same ID but different values
        updated = TestEntity(id=1, name="changed", value=200)
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
        repository: TestRepository,
    ) -> None:
        """Verifies repository can persist multiple new entities at once."""
        entities = [
            TestEntity(id=1, name="first", value=10),
            TestEntity(id=2, name="second", value=20),
            TestEntity(id=3, name="third", value=30),
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
        repository: TestRepository,
    ) -> None:
        """Verifies repository updates existing entities in bulk."""
        # Create initial entities
        initial = [
            TestEntity(id=1, name="first", value=10),
            TestEntity(id=2, name="second", value=20),
        ]
        await repository.save_bulk(initial)

        # Update the same entities
        updated = [
            TestEntity(id=1, name="updated_first", value=100),
            TestEntity(id=2, name="updated_second", value=200),
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
        repository: TestRepository,
    ) -> None:
        """Verifies repository can handle both new and existing entities in one call."""
        # Create one existing entity
        existing = TestEntity(id=1, name="existing", value=10)
        await repository.save(existing)

        # Mix of update and new entities
        mixed = [
            TestEntity(id=1, name="updated", value=100),  # Update existing
            TestEntity(id=2, name="new_one", value=20),  # New
            TestEntity(id=3, name="new_two", value=30),  # New
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
        repository: TestRepository,
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
        repository: TestRepository,
    ) -> None:
        """Verifies repository can delete an existing entity."""
        # Create entity
        entity = TestEntity(id=1, name="to_delete", value=42)
        await repository.save(entity)

        # Verify it exists
        assert await repository.exists(1)

        # Delete it
        await repository.delete(entity)

        # Verify it's gone
        assert not await repository.exists(1)

    async def test_deletes_nonexistent_entity_gracefully(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies deletion of non-existent entity doesn't cause error."""
        entity = TestEntity(id=999, name="nonexistent", value=0)

        # Should not raise an error
        await repository.delete(entity)

        # Verify it still doesn't exist
        assert not await repository.exists(999)

    async def test_deletes_correct_entity_when_multiple_exist(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies only the specified entity is deleted."""
        # Create multiple entities
        entities = [
            TestEntity(id=1, name="keep", value=10),
            TestEntity(id=2, name="delete_me", value=20),
            TestEntity(id=3, name="keep_too", value=30),
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


class TestDeleteBulk:
    """Tests for the delete_bulk method."""

    async def test_deletes_multiple_entities(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies repository can delete multiple entities at once."""
        # Create entities
        entities = [
            TestEntity(id=1, name="first", value=10),
            TestEntity(id=2, name="second", value=20),
            TestEntity(id=3, name="third", value=30),
        ]
        await repository.save_bulk(entities)

        # Delete all
        await repository.delete_bulk(entities)

        # Verify all are gone
        assert not await repository.exists(1)
        assert not await repository.exists(2)
        assert not await repository.exists(3)

        query = QueryBuilder()
        remaining = await repository.find(query)
        assert len(remaining) == 0

    async def test_deletes_subset_of_entities(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies bulk deletion of a subset of entities."""
        # Create entities
        all_entities = [
            TestEntity(id=1, name="keep", value=10),
            TestEntity(id=2, name="delete", value=20),
            TestEntity(id=3, name="delete_too", value=30),
            TestEntity(id=4, name="keep_too", value=40),
        ]
        await repository.save_bulk(all_entities)

        # Delete only middle two
        to_delete = [all_entities[1], all_entities[2]]
        await repository.delete_bulk(to_delete)

        # Verify correct entities were deleted
        assert await repository.exists(1)
        assert not await repository.exists(2)
        assert not await repository.exists(3)
        assert await repository.exists(4)

        query = QueryBuilder()
        remaining = await repository.find(query)
        assert len(remaining) == 2

    async def test_handles_empty_delete_list(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies bulk deletion handles empty lists gracefully."""
        # Create an entity
        entity = TestEntity(id=1, name="survivor", value=10)
        await repository.save(entity)

        # Delete empty list
        await repository.delete_bulk([])

        # Verify entity still exists
        assert await repository.exists(1)

    async def test_handles_nonexistent_entities_in_bulk_delete(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies bulk deletion handles non-existent entities gracefully."""
        entities = [
            TestEntity(id=1, name="nonexistent", value=10),
            TestEntity(id=2, name="also_nonexistent", value=20),
        ]

        # Should not raise an error
        await repository.delete_bulk(entities)

        # Verify they still don't exist
        assert not await repository.exists(1)
        assert not await repository.exists(2)


class TestFind:
    """Tests for the find method."""

    async def test_finds_all_entities_with_empty_query(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies find returns all entities when given empty query."""
        # Create entities
        entities = [
            TestEntity(id=1, name="first", value=10),
            TestEntity(id=2, name="second", value=20),
            TestEntity(id=3, name="third", value=30),
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
        repository: TestRepository,
    ) -> None:
        """Verifies find applies query filters correctly."""
        # Create entities with different values
        entities = [
            TestEntity(id=1, name="match", value=100),
            TestEntity(id=2, name="no_match", value=50),
            TestEntity(id=3, name="match", value=100),
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
        repository: TestRepository,
    ) -> None:
        """Verifies find respects limit in query."""
        # Create many entities
        entities = [TestEntity(id=i, name=f"entity_{i}", value=i) for i in range(1, 11)]
        await repository.save_bulk(entities)

        # Query with limit
        query = QueryBuilder().paginate(limit=5)
        found = await repository.find(query)

        assert len(found) == 5

    async def test_returns_empty_list_when_no_matches(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies find returns empty list when no entities match."""
        # Create entities
        entities = [TestEntity(id=1, name="test", value=10)]
        await repository.save_bulk(entities)

        # Query that won't match
        query = QueryBuilder().filter("value", FilterOperator.EQ, 999)
        found = await repository.find(query)

        assert found == []

    async def test_returns_empty_list_when_table_empty(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies find returns empty list when table is empty."""
        query = QueryBuilder()
        found = await repository.find(query)

        assert found == []


class TestExists:
    """Tests for the exists method."""

    async def test_returns_true_for_existing_entity(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies exists returns True for existing entity."""
        entity = TestEntity(id=1, name="test", value=42)
        await repository.save(entity)

        assert await repository.exists(1)

    async def test_returns_false_for_nonexistent_entity(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies exists returns False for non-existent entity."""
        assert not await repository.exists(999)

    async def test_returns_false_after_deletion(
        self,
        repository: TestRepository,
    ) -> None:
        """Verifies exists returns False after entity is deleted."""
        entity = TestEntity(id=1, name="test", value=42)
        await repository.save(entity)

        assert await repository.exists(1)

        await repository.delete(entity)

        assert not await repository.exists(1)
