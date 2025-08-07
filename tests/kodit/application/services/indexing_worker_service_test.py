"""Tests for the IndexingWorkerService."""

import asyncio
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.indexing_worker_service import IndexingWorkerService
from kodit.application.services.queue_service import QueueService
from kodit.config import AppContext
from kodit.domain.entities import File, Index, Source, Task, WorkingCopy
from kodit.domain.value_objects import (
    FileProcessingStatus,
    QueuePriority,
    SourceType,
    TaskType,
)


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
    session: AsyncSession,
    dummy_index: Index,
) -> None:
    """Test that the worker processes a task from the queue."""
    # Add a task to the queue
    queue_service = QueueService(session)
    task = Task.create_index_update_task(
        index_id=dummy_index.id, priority=QueuePriority.USER_INITIATED
    )
    await queue_service.enqueue_task(task)

    # Create worker service
    worker = IndexingWorkerService(app_context, session_factory)

    # Mock the indexing service
    with patch(
        "kodit.application.services.indexing_worker_service.create_code_indexing_application_service"
    ) as mock_create_service:
        mock_service = AsyncMock()
        mock_service.index_repository.get.return_value = dummy_index
        mock_service.run_index = AsyncMock()
        mock_create_service.return_value = mock_service

        # Start the worker
        await worker.start()

        # Give the worker time to process the task
        await asyncio.sleep(0.1)

        # Stop the worker
        await worker.stop()

        # Verify the task was processed
        mock_service.run_index.assert_called_once_with(dummy_index)


@pytest.mark.asyncio
async def test_worker_handles_missing_index(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
    session: AsyncSession,
) -> None:
    """Test that the worker handles missing index gracefully."""
    # Add a task with non-existent index
    queue_service = QueueService(session)
    task = Task.create_index_update_task(
        index_id=999,  # Non-existent
        priority=QueuePriority.USER_INITIATED,
    )
    await queue_service.enqueue_task(task)

    # Create worker service
    worker = IndexingWorkerService(app_context, session_factory)

    # Mock the indexing service
    with patch(
        "kodit.application.services.indexing_worker_service.create_code_indexing_application_service"
    ) as mock_create_service:
        mock_service = AsyncMock()
        mock_service.index_repository.get.return_value = None  # Index not found
        mock_create_service.return_value = mock_service

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
    session: AsyncSession,
) -> None:
    """Test that the worker handles invalid task payload gracefully."""
    # Add a task with invalid payload
    QueueService(session)
    task = Task(
        id="test-task-1",
        type=TaskType.INDEX_UPDATE,
        payload={},  # Missing index_id
        priority=QueuePriority.USER_INITIATED,
    )

    # Manually add the task to bypass validation
    from kodit.infrastructure.sqlalchemy.task_repository import SqlAlchemyTaskRepository

    repo = SqlAlchemyTaskRepository(session)
    await repo.add(task)
    await session.commit()

    # Create worker service
    worker = IndexingWorkerService(app_context, session_factory)

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
    session: AsyncSession,
) -> None:
    """Test that the worker processes multiple tasks sequentially."""
    # Add multiple tasks to the queue
    queue_service = QueueService(session)
    tasks = []
    for i in range(3):
        task = Task.create_index_update_task(
            index_id=i + 1,  # Use different index IDs to avoid deduplication
            priority=QueuePriority.BACKGROUND,
        )
        tasks.append(task)
        await queue_service.enqueue_task(task)

    # Create worker service
    worker = IndexingWorkerService(app_context, session_factory)

    # Track processing order
    processed_tasks = []

    async def mock_run_index(index: Index) -> None:
        processed_tasks.append(index.id)
        # No sleep needed for testing

    # Mock the indexing service
    with patch(
        "kodit.application.services.indexing_worker_service.create_code_indexing_application_service"
    ) as mock_create_service:
        mock_service = AsyncMock()

        # Mock to return a different index for each ID
        def mock_get_index(index_id: int) -> Index | None:
            index = MagicMock(spec=Index)
            index.id = index_id
            return index

        mock_service.index_repository.get.side_effect = mock_get_index
        mock_service.run_index = mock_run_index
        mock_create_service.return_value = mock_service

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
    worker = IndexingWorkerService(app_context, session_factory)

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
    session: AsyncSession,
) -> None:
    """Test that the worker continues processing after encountering an error."""
    # Add tasks to the queue
    queue_service = QueueService(session)

    # First task will succeed
    task1 = Task.create_index_update_task(
        index_id=1, priority=QueuePriority.USER_INITIATED
    )
    await queue_service.enqueue_task(task1)

    # Second task will fail
    task2 = Task.create_index_update_task(index_id=2, priority=QueuePriority.BACKGROUND)
    await queue_service.enqueue_task(task2)

    # Third task will succeed
    task3 = Task.create_index_update_task(index_id=3, priority=QueuePriority.BACKGROUND)
    await queue_service.enqueue_task(task3)

    # Create worker service
    worker = IndexingWorkerService(app_context, session_factory)

    # Track processed tasks
    processed_ids = []

    async def mock_run_index(index: Index) -> None:
        if index.id == 2:

            class TestError(Exception):
                pass

            raise TestError("Test error")
        processed_ids.append(index.id)

    # Mock the indexing service
    with patch(
        "kodit.application.services.indexing_worker_service.create_code_indexing_application_service"
    ) as mock_create_service:
        mock_service = AsyncMock()

        # Return different dummy indexes for each task
        def get_index(index_id: int) -> Index | None:
            # Return a dummy index for any ID
            index = MagicMock(spec=Index)
            index.id = index_id
            return index

        mock_service.index_repository.get.side_effect = get_index
        mock_service.run_index = mock_run_index
        mock_create_service.return_value = mock_service

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
    session: AsyncSession,
) -> None:
    """Test that the worker processes tasks in priority order."""
    # Add tasks with different priorities
    queue_service = QueueService(session)

    # Add in reverse priority order
    background_task = Task.create_index_update_task(
        index_id=1, priority=QueuePriority.BACKGROUND
    )
    user_task = Task.create_index_update_task(
        index_id=2, priority=QueuePriority.USER_INITIATED
    )

    await queue_service.enqueue_task(background_task)
    await queue_service.enqueue_task(user_task)

    # Create worker service
    worker = IndexingWorkerService(app_context, session_factory)

    # Track processing order
    processed_order = []

    async def mock_run_index(index: Index) -> None:
        processed_order.append(index.id)

    # Mock the indexing service
    with patch(
        "kodit.application.services.indexing_worker_service.create_code_indexing_application_service"
    ) as mock_create_service:
        mock_service = AsyncMock()

        def get_index(index_id: int) -> Index | None:
            index = MagicMock(spec=Index)
            index.id = index_id
            return index

        mock_service.index_repository.get.side_effect = get_index
        mock_service.run_index = mock_run_index
        mock_create_service.return_value = mock_service

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
