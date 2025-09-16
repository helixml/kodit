"""Tests for the IndexingWorkerService."""

import asyncio
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.server_factory import ServerFactory
from kodit.application.services.indexing_worker_service import IndexingWorkerService
from kodit.application.services.queue_service import QueueService
from kodit.config import AppContext
from kodit.domain.entities import File, Index, Source, Task, WorkingCopy
from kodit.domain.value_objects import (
    FileProcessingStatus,
    QueuePriority,
    SourceType,
    TaskOperation,
)
from kodit.infrastructure.sqlalchemy.task_repository import create_task_repository


@pytest.fixture
def session_factory(session: AsyncSession) -> Callable[[], AsyncSession]:
    """Create a session factory for the worker service."""

    # Return a simple callable that returns the session directly
    # The session itself is already an async context manager
    def factory() -> AsyncSession:
        return session

    return factory


@pytest.fixture
def dummy_index(tmp_path: Path) -> Index:
    """Create a dummy index for testing."""
    file = File(
        id=1,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        uri=AnyUrl("file:///test/file.py"),
        sha256="abc123",
        authors=[],
        mime_type="text/x-python",
        file_processing_status=FileProcessingStatus.CLEAN,
    )

    working_copy = WorkingCopy(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        cloned_path=tmp_path / "test-repo",
        source_type=SourceType.GIT,
        files=[file],
    )

    source = Source(
        id=1,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        working_copy=working_copy,
    )

    return Index(
        id=1,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source=source,
        snippets=[],
    )


@pytest.mark.asyncio
async def test_worker_processes_task(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
    dummy_index: Index,
) -> None:
    """Test that the worker processes a task from the queue."""
    # Add a task to the queue
    queue_service = QueueService(session_factory=session_factory)
    task = Task.create(
        TaskOperation.REFRESH_WORKING_COPY,
        QueuePriority.USER_INITIATED,
        {"index_id": dummy_index.id},
    )
    await queue_service.enqueue_task(task)

    # Create worker service
    # Create worker service with mocked server factory
    server_factory = MagicMock(spec=ServerFactory)
    mock_service = AsyncMock()
    mock_service.run_task = AsyncMock()
    server_factory.code_indexing_application_service.return_value = mock_service
    server_factory.commit_indexing_application_service.return_value = mock_service
    worker = IndexingWorkerService(app_context, session_factory, server_factory)

    # Start the worker
    await worker.start()

    # Give the worker time to process the task
    await asyncio.sleep(0.1)

    # Stop the worker
    await worker.stop()

    # Verify the task was processed
    mock_service.run_task.assert_called()


@pytest.mark.asyncio
async def test_worker_handles_missing_index(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that the worker handles missing index gracefully."""
    # Add a task with non-existent index
    queue_service = QueueService(session_factory=session_factory)
    task = Task.create(
        TaskOperation.REFRESH_WORKING_COPY,
        QueuePriority.USER_INITIATED,
        {"index_id": 999},  # Non-existent
    )
    await queue_service.enqueue_task(task)

    # Create worker service
    # Create worker service with mocked server factory
    server_factory = MagicMock(spec=ServerFactory)
    mock_service = AsyncMock()
    mock_service.run_task = AsyncMock()
    server_factory.code_indexing_application_service.return_value = mock_service
    server_factory.commit_indexing_application_service.return_value = mock_service
    worker = IndexingWorkerService(app_context, session_factory, server_factory)

    # Start the worker
    await worker.start()

    # Give the worker time to process the task
    await asyncio.sleep(0.1)

    # Stop the worker
    await worker.stop()

    # Worker should have handled the error and continued


@pytest.mark.asyncio
async def test_worker_handles_invalid_task_payload(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that the worker handles invalid task payload gracefully."""
    # Add a task with invalid payload
    task = Task(
        id="test-task-1",
        type=TaskOperation.REFRESH_WORKING_COPY,
        payload={},  # Missing index_id
        priority=QueuePriority.USER_INITIATED,
    )

    repo = create_task_repository(session_factory=session_factory)
    await repo.add(task)

    # Create worker service
    # Create worker service with mocked server factory
    server_factory = MagicMock(spec=ServerFactory)
    mock_service = AsyncMock()
    mock_service.run_task = AsyncMock()
    server_factory.code_indexing_application_service.return_value = mock_service
    server_factory.commit_indexing_application_service.return_value = mock_service
    worker = IndexingWorkerService(app_context, session_factory, server_factory)

    # Start the worker
    await worker.start()

    # Give the worker time to process the task
    await asyncio.sleep(0.1)

    # Stop the worker
    await worker.stop()

    # Worker should have handled the error and continued


@pytest.mark.asyncio
async def test_worker_processes_multiple_tasks_sequentially(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that the worker processes multiple tasks sequentially."""
    # Add multiple tasks to the queue
    queue_service = QueueService(session_factory=session_factory)
    tasks = []
    for i in range(3):
        task = Task.create(
            TaskOperation.REFRESH_WORKING_COPY,
            QueuePriority.BACKGROUND,
            {"index_id": i + 1},  # Use different index IDs to avoid deduplication
        )
        tasks.append(task)
        await queue_service.enqueue_task(task)

    async def mock_run_task(task: Task) -> None:
        index_id = task.payload["index_id"]
        processed_tasks.append(index_id)
        # No sleep needed for testing

    # Create worker service with mocked server factory
    server_factory = MagicMock(spec=ServerFactory)
    mock_service = AsyncMock()
    mock_service.run_task = AsyncMock(side_effect=mock_run_task)
    server_factory.code_indexing_application_service.return_value = mock_service
    server_factory.commit_indexing_application_service.return_value = mock_service
    worker = IndexingWorkerService(app_context, session_factory, server_factory)

    # Track processing order
    processed_tasks = []

    # Start the worker
    await worker.start()

    # Wait for all tasks to be processed
    for _ in range(30):  # Wait up to 3 seconds
        if len(processed_tasks) >= 3:
            break
        await asyncio.sleep(0.1)

    # Stop the worker
    await worker.stop()

    # Verify all tasks were processed
    assert len(processed_tasks) == 3


@pytest.mark.asyncio
async def test_worker_stops_gracefully(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that the worker stops gracefully when requested."""
    # Create worker service
    # Create worker service with mocked server factory
    server_factory = MagicMock(spec=ServerFactory)
    mock_service = AsyncMock()
    mock_service.run_task = AsyncMock()
    server_factory.code_indexing_application_service.return_value = mock_service
    server_factory.commit_indexing_application_service.return_value = mock_service
    worker = IndexingWorkerService(app_context, session_factory, server_factory)

    # Start the worker
    await worker.start()

    # Verify the worker task is running
    assert worker._worker_task is not None  # noqa: SLF001
    assert not worker._worker_task.done()  # noqa: SLF001

    # Stop the worker
    await worker.stop()

    # Verify the worker task has stopped
    assert worker._worker_task.done()  # noqa: SLF001


@pytest.mark.asyncio
async def test_worker_continues_after_error(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that the worker continues processing after encountering an error."""
    # Add tasks to the queue
    queue_service = QueueService(session_factory=session_factory)

    # First task will succeed
    task1 = Task.create(
        TaskOperation.REFRESH_WORKING_COPY,
        QueuePriority.USER_INITIATED,
        {"index_id": 1},
    )
    await queue_service.enqueue_task(task1)

    # Second task will fail
    task2 = Task.create(
        TaskOperation.REFRESH_WORKING_COPY, QueuePriority.BACKGROUND, {"index_id": 2}
    )
    await queue_service.enqueue_task(task2)

    # Third task will succeed
    task3 = Task.create(
        TaskOperation.REFRESH_WORKING_COPY, QueuePriority.BACKGROUND, {"index_id": 3}
    )
    await queue_service.enqueue_task(task3)

    async def mock_run_task(task: Task) -> None:
        index_id = task.payload["index_id"]
        if index_id == 2:

            class TestError(Exception):
                pass

            raise TestError("Test error")
        processed_ids.append(index_id)

    # Create worker service with mocked server factory
    server_factory = MagicMock(spec=ServerFactory)
    mock_service = AsyncMock()
    mock_service.run_task = AsyncMock(side_effect=mock_run_task)
    server_factory.code_indexing_application_service.return_value = mock_service
    server_factory.commit_indexing_application_service.return_value = mock_service
    worker = IndexingWorkerService(app_context, session_factory, server_factory)

    # Track processed tasks
    processed_ids = []

    # Start the worker
    await worker.start()

    # Wait for tasks to be processed (may include failures)
    for _ in range(50):  # Wait up to 5 seconds
        # We expect tasks 1 and 3 to be processed, but not 2
        if len(processed_ids) >= 2:
            break
        await asyncio.sleep(0.1)

    # Stop the worker
    await worker.stop()

    # Verify tasks 1 and 3 were processed despite task 2 failing
    assert 1 in processed_ids
    assert 3 in processed_ids
    assert 2 not in processed_ids  # This one failed


@pytest.mark.asyncio
async def test_worker_respects_task_priority(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that the worker processes tasks in priority order."""
    # Add tasks with different priorities
    queue_service = QueueService(session_factory=session_factory)

    # Add in reverse priority order
    background_task = Task.create(
        TaskOperation.REFRESH_WORKING_COPY, QueuePriority.BACKGROUND, {"index_id": 1}
    )
    user_task = Task.create(
        TaskOperation.REFRESH_WORKING_COPY,
        QueuePriority.USER_INITIATED,
        {"index_id": 2},
    )

    await queue_service.enqueue_task(background_task)
    await queue_service.enqueue_task(user_task)

    async def mock_run_task(task: Task) -> None:
        index_id = task.payload["index_id"]
        processed_order.append(index_id)

    # Create worker service with mocked server factory
    server_factory = MagicMock(spec=ServerFactory)
    mock_service = AsyncMock()
    mock_service.run_task = AsyncMock(side_effect=mock_run_task)
    server_factory.code_indexing_application_service.return_value = mock_service
    server_factory.commit_indexing_application_service.return_value = mock_service
    worker = IndexingWorkerService(app_context, session_factory, server_factory)

    # Track processing order
    processed_order = []

    # Start the worker
    await worker.start()

    # Wait for tasks to be processed
    for _ in range(50):  # Wait up to 5 seconds
        if len(processed_order) == 2:
            break
        await asyncio.sleep(0.1)

    # Stop the worker
    await worker.stop()

    # Verify both tasks were processed
    assert len(processed_order) == 2

    # Verify the task with the highest priority was processed first
    assert processed_order[0] == 2
    assert processed_order[1] == 1
