"""Tests for the queue service get_task method."""

from collections.abc import Callable

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.queue_service import QueueService
from kodit.domain.entities import Task
from kodit.domain.value_objects import QueuePriority, TaskOperation


async def test_get_task_existing(session_factory: Callable[[], AsyncSession]) -> None:
    """Test getting an existing task by ID."""
    queue_service = QueueService(session_factory=session_factory)

    # Create and enqueue a task
    task = Task.create(
        TaskOperation.REFRESH_WORKING_COPY,
        QueuePriority.USER_INITIATED,
        {"index_id": 1},
    )
    await queue_service.enqueue_task(task)

    # Get the task by ID
    retrieved_task = await queue_service.get_task(task.id)

    assert retrieved_task is not None
    assert retrieved_task.id == task.id
    assert retrieved_task.type == TaskOperation.REFRESH_WORKING_COPY
    assert retrieved_task.payload["index_id"] == 1
    assert retrieved_task.priority == QueuePriority.USER_INITIATED


async def test_get_task_nonexistent(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test getting a non-existent task returns None."""
    queue_service = QueueService(session_factory=session_factory)

    # Try to get a task that doesn't exist
    retrieved_task = await queue_service.get_task("nonexistent-id")

    assert retrieved_task is None


async def test_get_task_after_multiple_tasks(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test getting a specific task when multiple tasks exist."""
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

    # Get a specific task by ID
    target_task = tasks[1]
    retrieved_task = await queue_service.get_task(target_task.id)

    assert retrieved_task is not None
    assert retrieved_task.id == target_task.id
    assert retrieved_task.payload["index_id"] == 1
    assert retrieved_task.priority == QueuePriority.USER_INITIATED
