"""Tests for SqlAlchemyTaskRepository."""

from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Task
from kodit.domain.value_objects import TaskOperation
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder
from kodit.infrastructure.sqlalchemy.task_repository import create_task_repository


async def test_add_and_get_task(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test adding and retrieving a task."""
    repository = create_task_repository(session_factory)
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )

    await repository.save(task)

    loaded = await repository.get(task.id)
    assert loaded is not None
    assert loaded.id == task.id


async def test_next_returns_highest_priority(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that next() returns the highest priority task."""
    repository = create_task_repository(session_factory)

    low_priority = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=5,
        payload={"index_id": 1},
    )
    high_priority = Task.create(
        operation=TaskOperation.ENRICH_SNIPPETS,
        priority=100,
        payload={"index_id": 2},
    )

    await repository.save(low_priority)
    await repository.save(high_priority)

    next_task = await repository.next()
    assert next_task is not None
    assert next_task.id == high_priority.id


async def test_remove_task(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test removing a task."""
    repository = create_task_repository(session_factory)
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )

    await repository.save(task)
    await repository.delete(task)

    exists = await repository.exists(task.id)
    assert not exists


async def test_update_task(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test updating a task."""
    repository = create_task_repository(session_factory)
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )

    await repository.save(task)

    task.priority = 50
    await repository.save(task)

    loaded = await repository.get(task.id)
    assert loaded is not None
    assert loaded.priority == 50


async def test_list_tasks(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test listing tasks."""
    repository = create_task_repository(session_factory)

    task1 = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )
    task2 = Task.create(
        operation=TaskOperation.EXTRACT_SNIPPETS,
        priority=5,
        payload={"index_id": 2},
    )

    await repository.save(task1)
    await repository.save(task2)

    tasks = await repository.find(QueryBuilder())
    assert len(tasks) == 2


async def test_list_tasks_with_filter(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test listing tasks with operation filter."""
    repository = create_task_repository(session_factory)

    task1 = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )
    task2 = Task.create(
        operation=TaskOperation.EXTRACT_SNIPPETS,
        priority=5,
        payload={"index_id": 2},
    )

    await repository.save(task1)
    await repository.save(task2)

    query = QueryBuilder().filter("type", FilterOperator.EQ, TaskOperation.CREATE_INDEX)
    tasks = await repository.find(query)
    assert len(tasks) == 1
    assert tasks[0].type == TaskOperation.CREATE_INDEX
