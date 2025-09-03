"""Tests for SqlAlchemyTaskRepository."""

from unittest.mock import AsyncMock

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Task
from kodit.domain.value_objects import QueuePriority, TaskType
from kodit.infrastructure.sqlalchemy import entities as db_entities
from kodit.infrastructure.sqlalchemy.task_repository import SqlAlchemyTaskRepository
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


class TestAdd:
    """Test cases for the add method."""

    @pytest.mark.asyncio
    async def test_adds_task_to_database(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that add method adds a task to the database."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)
        task = Task.create_index_update_task(
            index_id=123, priority=QueuePriority.USER_INITIATED
        )

        # Act
        await repository.add(task)

        # Assert - verify task was added by trying to retrieve it
        retrieved_task = await repository.get("123")
        assert retrieved_task is not None
        assert retrieved_task.id == "123"
        assert retrieved_task.type == TaskType.INDEX_UPDATE
        assert retrieved_task.priority == QueuePriority.USER_INITIATED
        assert retrieved_task.payload == {"index_id": 123}

    @pytest.mark.asyncio
    async def test_calls_session_add_with_mapped_entity(self) -> None:
        """Test that add method calls session.add with the mapped entity."""
        # Arrange
        mock_session = AsyncMock(spec=AsyncSession)
        mock_uow = AsyncMock(spec=SqlAlchemyUnitOfWork)
        mock_uow.session = mock_session
        mock_uow.__aenter__ = AsyncMock(return_value=mock_uow)
        mock_uow.__aexit__ = AsyncMock(return_value=None)

        repository = SqlAlchemyTaskRepository(mock_uow)
        task = Task.create_index_update_task(
            index_id=456, priority=QueuePriority.BACKGROUND
        )

        # Act
        await repository.add(task)

        # Assert
        mock_session.add.assert_called_once()
        added_entity = mock_session.add.call_args[0][0]
        assert isinstance(added_entity, db_entities.Task)
        assert added_entity.dedup_key == "456"
        assert added_entity.type == db_entities.TaskType.INDEX_UPDATE
        assert added_entity.priority == QueuePriority.BACKGROUND


class TestGet:
    """Test cases for the get method."""

    @pytest.mark.asyncio
    async def test_gets_task_from_database(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that get method retrieves an existing task from database."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)
        task = Task.create_index_update_task(
            index_id=999, priority=QueuePriority.BACKGROUND
        )
        await repository.add(task)

        # Act
        retrieved_task = await repository.get("999")

        # Assert
        assert retrieved_task is not None
        assert retrieved_task.id == "999"
        assert retrieved_task.type == TaskType.INDEX_UPDATE
        assert retrieved_task.priority == QueuePriority.BACKGROUND
        assert retrieved_task.payload == {"index_id": 999}

    @pytest.mark.asyncio
    async def test_returns_none_if_task_does_not_exist(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that get method returns None when task doesn't exist."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)

        # Act
        result = await repository.get("non-existent-id")

        # Assert
        assert result is None


class TestTake:
    """Test cases for the take method."""

    @pytest.mark.asyncio
    async def test_takes_task_from_database(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that take method retrieves and removes a task from database."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)
        task = Task.create_index_update_task(
            index_id=777, priority=QueuePriority.USER_INITIATED
        )
        await repository.add(task)

        # Act
        taken_task = await repository.take()

        # Assert
        assert taken_task is not None
        assert taken_task.id == "777"

        # Verify task was removed
        remaining_task = await repository.get("777")
        assert remaining_task is None

    @pytest.mark.asyncio
    async def test_returns_none_if_no_tasks_available(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that take method returns None when no tasks are available."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)

        # Act
        result = await repository.take()

        # Assert
        assert result is None

    @pytest.mark.asyncio
    async def test_takes_highest_priority_task_first(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that take method returns the highest priority task."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)

        # Add tasks with different priorities (using different index IDs)
        low_priority_task = Task(
            id="1",
            type=TaskType.INDEX_UPDATE,
            priority=1,
            payload={"index_id": 1},
        )
        high_priority_task = Task(
            id="2",
            type=TaskType.INDEX_UPDATE,
            priority=100,
            payload={"index_id": 2},
        )
        medium_priority_task = Task(
            id="3",
            type=TaskType.INDEX_UPDATE,
            priority=50,
            payload={"index_id": 3},
        )

        await repository.add(low_priority_task)
        await repository.add(high_priority_task)
        await repository.add(medium_priority_task)

        # Act
        first_taken = await repository.take()
        second_taken = await repository.take()
        third_taken = await repository.take()

        # Assert
        assert first_taken is not None
        assert second_taken is not None
        assert third_taken is not None
        assert first_taken.id == "2"  # highest priority
        assert second_taken.id == "3"  # medium priority
        assert third_taken.id == "1"  # lowest priority


class TestUpdate:
    """Test cases for the update method."""

    @pytest.mark.asyncio
    async def test_updates_task_in_database(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that update method modifies an existing task."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)
        original_task = Task.create_index_update_task(
            index_id=888, priority=QueuePriority.BACKGROUND
        )
        await repository.add(original_task)

        # Modify the task
        updated_task = Task(
            id="888",
            type=TaskType.INDEX_UPDATE,  # Type cannot be updated
            priority=100,  # Updated priority
            payload={"index_id": 888, "modified": "data"},  # Updated payload
        )

        # Act
        await repository.update(updated_task)

        # Assert
        retrieved = await repository.get("888")
        assert retrieved is not None
        assert retrieved.priority == 100
        assert retrieved.payload == {"index_id": 888, "modified": "data"}
        assert retrieved.type == TaskType.INDEX_UPDATE  # Type unchanged

    @pytest.mark.asyncio
    async def test_raises_value_error_if_task_does_not_exist(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that update raises ValueError for non-existent task."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)
        non_existent_task = Task(
            id="does-not-exist",
            type=TaskType.INDEX_UPDATE,
            priority=5,
            payload={"index_id": 999},
        )

        # Act & Assert
        with pytest.raises(ValueError, match="Task not found: does-not-exist"):
            await repository.update(non_existent_task)


class TestList:
    """Test cases for the list method."""

    @pytest.mark.asyncio
    async def test_lists_all_tasks_in_database(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that list method returns all tasks."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)
        tasks = [
            Task(
                id=str(i + 100),
                type=TaskType.INDEX_UPDATE,
                priority=i * 10,
                payload={"index_id": i + 100},
            )
            for i in range(3)
        ]
        for task in tasks:
            await repository.add(task)

        # Act
        result = await repository.list()

        # Assert
        assert len(result) == 3
        task_ids = {task.id for task in result}
        assert task_ids == {"100", "101", "102"}

    @pytest.mark.asyncio
    async def test_returns_empty_list_if_no_tasks(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that list method returns empty list when no tasks exist."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)

        # Act
        result = await repository.list()

        # Assert
        assert result == []

    @pytest.mark.asyncio
    async def test_filters_tasks_by_type_if_provided(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that list method filters by task type when specified."""
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)

        # Since we only have one task type, we'll create multiple tasks and verify
        # filtering works
        task1 = Task.create_index_update_task(
            index_id=500, priority=QueuePriority.USER_INITIATED
        )
        task2 = Task.create_index_update_task(
            index_id=501, priority=QueuePriority.BACKGROUND
        )

        await repository.add(task1)
        await repository.add(task2)

        # Act
        all_tasks = await repository.list()
        filtered_tasks = await repository.list(task_type=TaskType.INDEX_UPDATE)

        # Assert
        assert len(all_tasks) == 2
        assert len(filtered_tasks) == 2  # Should be same since all are INDEX_UPDATE
        task_ids = {task.id for task in filtered_tasks}
        assert task_ids == {"500", "501"}

    @pytest.mark.asyncio
    async def test_sorts_tasks_by_priority_and_created_at(
        self, unit_of_work: SqlAlchemyUnitOfWork
    ) -> None:
        """Test that list method returns tasks by priority (desc) then created_at.

        Test priority descending, then created_at ascending for equal priorities.
        """
        # Arrange
        repository = SqlAlchemyTaskRepository(unit_of_work)

        # Create tasks with specific priorities
        # Different priorities to test sorting
        task1 = Task(
            id="task-1",
            type=TaskType.INDEX_UPDATE,
            priority=5,
            payload={"index_id": 1},
        )
        task2 = Task(
            id="task-2",
            type=TaskType.INDEX_UPDATE,
            priority=5,
            payload={"index_id": 2},
        )
        # Higher priority
        task3 = Task(
            id="task-3",
            type=TaskType.INDEX_UPDATE,
            priority=100,
            payload={"index_id": 3},
        )
        # Lower priority
        task4 = Task(
            id="task-4",
            type=TaskType.INDEX_UPDATE,
            priority=1,
            payload={"index_id": 4},
        )

        # Add in random order
        await repository.add(task2)
        await repository.add(task4)
        await repository.add(task1)
        await repository.add(task3)

        # Act
        result = await repository.list()

        # Assert
        priorities = [task.priority for task in result]
        # Should be ordered by priority descending: 100, 5, 5, 1
        assert priorities == [100, 5, 5, 1]
        # The first task should be the highest priority
        assert result[0].id == "task-3"
        # The last task should be the lowest priority
        assert result[-1].id == "task-4"
