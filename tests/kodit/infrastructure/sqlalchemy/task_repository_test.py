"""Tests for SqlAlchemyTaskRepository."""

from collections.abc import Callable
from datetime import UTC, datetime, timedelta

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import Task
from kodit.domain.value_objects import TaskOperation
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder
from kodit.infrastructure.sqlalchemy.task_repository import create_task_repository

# Used in retry tests below
_ = (datetime, UTC, timedelta)


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


async def test_next_skips_tasks_with_future_retry_time(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that next() skips tasks with next_retry_at in the future."""
    repository = create_task_repository(session_factory)

    # Create a task with next_retry_at in the future
    future_task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=100,
        payload={"index_id": 1},
    )
    future_task.next_retry_at = datetime.now(UTC) + timedelta(hours=1)
    await repository.save(future_task)

    # Create a task that's ready to run (no next_retry_at)
    ready_task = Task.create(
        operation=TaskOperation.EXTRACT_SNIPPETS,
        priority=10,
        payload={"index_id": 2},
    )
    await repository.save(ready_task)

    # next() should return the ready task, not the higher-priority future task
    next_task = await repository.next()
    assert next_task is not None
    assert next_task.id == ready_task.id


async def test_next_returns_task_with_past_retry_time(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that next() returns tasks with next_retry_at in the past."""
    repository = create_task_repository(session_factory)

    # Create a task with next_retry_at in the past
    past_retry_task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=100,
        payload={"index_id": 1},
    )
    past_retry_task.next_retry_at = datetime.now(UTC) - timedelta(minutes=1)
    past_retry_task.retry_count = 3
    await repository.save(past_retry_task)

    # next() should return this task since retry time has passed
    next_task = await repository.next()
    assert next_task is not None
    assert next_task.id == past_retry_task.id
    assert next_task.retry_count == 3


async def test_next_returns_none_when_all_tasks_pending_retry(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that next() returns None when all tasks are waiting for retry."""
    repository = create_task_repository(session_factory)

    # Create only tasks with future retry times
    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=100,
        payload={"index_id": 1},
    )
    task.next_retry_at = datetime.now(UTC) + timedelta(hours=1)
    await repository.save(task)

    # next() should return None since no tasks are ready
    next_task = await repository.next()
    assert next_task is None


async def test_retry_fields_persist_after_save(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that retry_count and next_retry_at are persisted correctly."""
    repository = create_task_repository(session_factory)

    task = Task.create(
        operation=TaskOperation.CREATE_INDEX,
        priority=10,
        payload={"index_id": 1},
    )
    await repository.save(task)

    # Mark for retry and save
    task.mark_for_retry()
    await repository.save(task)

    # Load and verify
    loaded = await repository.get(task.id)
    assert loaded.retry_count == 1
    assert loaded.next_retry_at is not None
    assert loaded.next_retry_at > datetime.now(UTC)
