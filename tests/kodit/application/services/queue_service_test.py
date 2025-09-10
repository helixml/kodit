"""Tests for the QueueService."""

from collections.abc import Callable

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.queue_service import QueueService
from kodit.domain.entities import Task
from kodit.domain.value_objects import QueuePriority, TaskOperation


@pytest.mark.asyncio
async def test_enqueue_task_new_task(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test enqueuing a new task."""
    queue_service = QueueService(session_factory=session_factory)

    # Create a test task
    task = Task.create(
        TaskOperation.REFRESH_WORKING_COPY,
        QueuePriority.USER_INITIATED,
        {"index_id": 1},
    )

    # Enqueue the task
    await queue_service.enqueue_task(task)

    # Verify the task was added
    tasks = await queue_service.list_tasks()
    assert len(tasks) == 1
    assert tasks[0].id == task.id
    assert tasks[0].type == TaskOperation.REFRESH_WORKING_COPY
    assert tasks[0].payload["index_id"] == 1
    assert tasks[0].priority == QueuePriority.USER_INITIATED


@pytest.mark.asyncio
async def test_enqueue_task_existing_task_updates_priority(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that enqueuing an existing task updates its priority."""
    queue_service = QueueService(session_factory=session_factory)

    # Create and enqueue a task with user-initiated priority
    task = Task.create(
        TaskOperation.REFRESH_WORKING_COPY,
        QueuePriority.USER_INITIATED,
        {"index_id": 1},
    )
    await queue_service.enqueue_task(task)

    # Create the same task with background priority
    background_priority_task = Task(
        id=task.id,
        type=TaskOperation.REFRESH_WORKING_COPY,
        payload={"index_id": 1},
        priority=QueuePriority.BACKGROUND,
    )
    await queue_service.enqueue_task(background_priority_task)

    # Verify only one task exists with updated priority
    tasks = await queue_service.list_tasks()
    assert len(tasks) == 1
    assert tasks[0].id == task.id
    assert tasks[0].priority == QueuePriority.BACKGROUND


@pytest.mark.asyncio
async def test_enqueue_multiple_tasks(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test enqueuing multiple tasks."""
    queue_service = QueueService(session_factory=session_factory)

    # Create and enqueue multiple tasks
    tasks = []
    for i in range(3):
        task = Task.create(
            TaskOperation.REFRESH_WORKING_COPY,
            QueuePriority.BACKGROUND if i % 2 == 0 else QueuePriority.USER_INITIATED,
            {"index_id": i},
        )
        tasks.append(task)
        await queue_service.enqueue_task(task)

    # Verify all tasks were added
    queued_tasks = await queue_service.list_tasks()
    assert len(queued_tasks) == 3

    # Verify task IDs match
    queued_ids = {t.id for t in queued_tasks}
    expected_ids = {t.id for t in tasks}
    assert queued_ids == expected_ids


@pytest.mark.asyncio
async def test_list_tasks_by_type(session_factory: Callable[[], AsyncSession]) -> None:
    """Test listing tasks filtered by type."""
    queue_service = QueueService(session_factory=session_factory)

    # Create tasks with different types
    # Test the filtering logic
    task1 = Task.create(
        TaskOperation.REFRESH_WORKING_COPY, QueuePriority.BACKGROUND, {"index_id": 1}
    )
    task2 = Task.create(
        TaskOperation.EXTRACT_SNIPPETS, QueuePriority.USER_INITIATED, {"index_id": 2}
    )

    await queue_service.enqueue_task(task1)
    await queue_service.enqueue_task(task2)

    # List all tasks
    all_tasks = await queue_service.list_tasks()
    assert len(all_tasks) == 2

    # List tasks by operation
    sync_tasks = await queue_service.list_tasks(
        task_operation=TaskOperation.REFRESH_WORKING_COPY
    )
    assert len(sync_tasks) == 1
    assert all(t.type == TaskOperation.REFRESH_WORKING_COPY for t in sync_tasks)

    extract_tasks = await queue_service.list_tasks(
        task_operation=TaskOperation.EXTRACT_SNIPPETS
    )
    assert len(extract_tasks) == 1
    assert all(t.type == TaskOperation.EXTRACT_SNIPPETS for t in extract_tasks)


@pytest.mark.asyncio
async def test_list_tasks_empty_queue(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test listing tasks when queue is empty."""
    queue_service = QueueService(session_factory=session_factory)

    tasks = await queue_service.list_tasks()
    assert len(tasks) == 0


@pytest.mark.asyncio
async def test_task_priority_ordering(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that tasks are returned in priority order."""
    queue_service = QueueService(session_factory=session_factory)

    # Create tasks with different priorities
    background_task = Task.create(
        TaskOperation.REFRESH_WORKING_COPY, QueuePriority.BACKGROUND, {"index_id": 1}
    )
    user_task = Task.create(
        TaskOperation.REFRESH_WORKING_COPY,
        QueuePriority.USER_INITIATED,
        {"index_id": 2},
    )

    # Enqueue in random order
    await queue_service.enqueue_task(user_task)
    await queue_service.enqueue_task(background_task)

    # List tasks and verify they are ordered by priority
    tasks = await queue_service.list_tasks()
    assert len(tasks) == 2

    # The repository should return tasks ordered by priority (highest first)
    task_priorities = [t.priority for t in tasks]
    assert task_priorities[0] >= task_priorities[1]
