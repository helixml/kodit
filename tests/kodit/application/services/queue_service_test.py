"""Tests for the QueueService."""

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.queue_service import QueueService
from kodit.domain.entities import Task
from kodit.domain.value_objects import QueuePriority, TaskType


@pytest.mark.asyncio
async def test_enqueue_task_new_task(session: AsyncSession) -> None:
    """Test enqueuing a new task."""
    queue_service = QueueService(session)

    # Create a test task
    task = Task.create_index_update_task(
        index_id=1, priority=QueuePriority.USER_INITIATED
    )

    # Enqueue the task
    await queue_service.enqueue_task(task)

    # Verify the task was added
    tasks = await queue_service.list_tasks()
    assert len(tasks) == 1
    assert tasks[0].id == task.id
    assert tasks[0].type == TaskType.INDEX_UPDATE
    assert tasks[0].payload["index_id"] == 1
    assert tasks[0].priority == QueuePriority.USER_INITIATED


@pytest.mark.asyncio
async def test_enqueue_task_existing_task_updates_priority(
    session: AsyncSession,
) -> None:
    """Test that enqueuing an existing task updates its priority."""
    queue_service = QueueService(session)

    # Create and enqueue a task with user-initiated priority
    task = Task.create_index_update_task(
        index_id=1, priority=QueuePriority.USER_INITIATED
    )
    await queue_service.enqueue_task(task)

    # Create the same task with background priority
    background_priority_task = Task(
        id=task.id,
        type=TaskType.INDEX_UPDATE,
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
async def test_enqueue_multiple_tasks(session: AsyncSession) -> None:
    """Test enqueuing multiple tasks."""
    queue_service = QueueService(session)

    # Create and enqueue multiple tasks
    tasks = []
    for i in range(3):
        task = Task.create_index_update_task(
            index_id=i,
            priority=QueuePriority.BACKGROUND
            if i % 2 == 0
            else QueuePriority.USER_INITIATED,
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
async def test_list_tasks_by_type(session: AsyncSession) -> None:
    """Test listing tasks filtered by type."""
    queue_service = QueueService(session)

    # Create tasks with different types
    # Currently only INDEX_UPDATE is supported, but test the filtering logic
    task1 = Task.create_index_update_task(index_id=1, priority=QueuePriority.BACKGROUND)
    task2 = Task.create_index_update_task(
        index_id=2, priority=QueuePriority.USER_INITIATED
    )

    await queue_service.enqueue_task(task1)
    await queue_service.enqueue_task(task2)

    # List all tasks
    all_tasks = await queue_service.list_tasks()
    assert len(all_tasks) == 2

    # List tasks by type
    index_tasks = await queue_service.list_tasks(task_type=TaskType.INDEX_UPDATE)
    assert len(index_tasks) == 2
    assert all(t.type == TaskType.INDEX_UPDATE for t in index_tasks)


@pytest.mark.asyncio
async def test_list_tasks_empty_queue(session: AsyncSession) -> None:
    """Test listing tasks when queue is empty."""
    queue_service = QueueService(session)

    tasks = await queue_service.list_tasks()
    assert len(tasks) == 0


@pytest.mark.asyncio
async def test_task_priority_ordering(session: AsyncSession) -> None:
    """Test that tasks are returned in priority order."""
    queue_service = QueueService(session)

    # Create tasks with different priorities
    background_task = Task.create_index_update_task(
        index_id=1, priority=QueuePriority.BACKGROUND
    )
    user_task = Task.create_index_update_task(
        index_id=2, priority=QueuePriority.USER_INITIATED
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
