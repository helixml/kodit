"""Tests for SyncSchedulerService."""

import asyncio
import contextlib
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.sync_scheduler import SyncSchedulerService
from kodit.domain.entities import File, Index, Source, WorkingCopy
from kodit.domain.value_objects import FileProcessingStatus, SourceType


@pytest.fixture
def mock_session_factory() -> Callable[[], AsyncSession]:
    """Create a mock session factory that returns AsyncSession-like objects."""
    session = AsyncMock(spec=AsyncSession)

    # Make session work as an async context manager
    session.__aenter__ = AsyncMock(return_value=session)
    session.__aexit__ = AsyncMock(return_value=None)

    # Create a factory that returns the session (synchronously)
    def session_factory() -> AsyncMock:
        return session

    return session_factory


@pytest.fixture
def dummy_indexes(tmp_path: Path) -> list[Index]:
    """Create dummy indexes for testing."""
    # Create dummy files
    file1 = File(
        id=1,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        uri=AnyUrl("file:///path/to/file1.py"),
        sha256="abc123",
        authors=[],
        mime_type="text/x-python",
        file_processing_status=FileProcessingStatus.CLEAN,
    )

    file2 = File(
        id=2,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        uri=AnyUrl("file:///path/to/file2.py"),
        sha256="def456",
        authors=[],
        mime_type="text/x-python",
        file_processing_status=FileProcessingStatus.CLEAN,
    )

    # Create working copy
    working_copy = WorkingCopy(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        cloned_path=tmp_path / "test-repo",
        source_type=SourceType.GIT,
        files=[file1, file2],
    )

    # Create source
    source = Source(
        id=1,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        working_copy=working_copy,
    )

    # Create indexes
    index1 = Index(
        id=1,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source=source,
        snippets=[],
    )

    index2 = Index(
        id=2,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        source=Source(
            id=2,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            working_copy=WorkingCopy(
                created_at=datetime.now(UTC),
                updated_at=datetime.now(UTC),
                remote_uri=AnyUrl("https://github.com/test/repo2.git"),
                cloned_path=tmp_path / "test-repo2",
                source_type=SourceType.GIT,
                files=[],
            ),
        ),
        snippets=[],
    )

    return [index1, index2]


@pytest.mark.asyncio
async def test_sync_scheduler_syncs_all_indexes(
    mock_session_factory: Callable[[], AsyncSession],
    dummy_indexes: list[Index],
) -> None:
    """Test that the sync scheduler syncs all existing indexes."""
    with (
        patch(
            "kodit.application.services.sync_scheduler.QueueService"
        ) as mock_queue_service_class,
        patch(
            "kodit.application.services.sync_scheduler.IndexQueryService"
        ) as mock_query_service_class,
        patch("kodit.application.services.sync_scheduler.SqlAlchemyIndexRepository"),
    ):
        # Set up mocks
        mock_queue_service = AsyncMock()
        mock_queue_service_class.return_value = mock_queue_service

        mock_query_service = AsyncMock()
        mock_query_service.list_indexes.return_value = dummy_indexes
        mock_query_service_class.return_value = mock_query_service

        # Create scheduler
        scheduler = SyncSchedulerService(mock_session_factory)

        # Perform one sync
        await scheduler._perform_sync()  # noqa: SLF001

        # Verify all indexes were synced
        assert mock_query_service.list_indexes.called
        assert mock_queue_service.enqueue_task.call_count == len(dummy_indexes)

        # Verify each index was enqueued with correct parameters
        for i, index in enumerate(dummy_indexes):
            call_args = mock_queue_service.enqueue_task.call_args_list[i]
            task = call_args[0][0]
            assert task.payload["index_id"] == index.id


@pytest.mark.asyncio
async def test_sync_scheduler_handles_empty_indexes(
    mock_session_factory: Callable[[], AsyncSession]
) -> None:
    """Test that the sync scheduler handles the case when no indexes exist."""
    with (
        patch(
            "kodit.application.services.sync_scheduler.QueueService"
        ) as mock_queue_service_class,
        patch(
            "kodit.application.services.sync_scheduler.IndexQueryService"
        ) as mock_query_service_class,
        patch("kodit.application.services.sync_scheduler.SqlAlchemyIndexRepository"),
    ):
        # Set up mocks for no indexes
        mock_queue_service = AsyncMock()
        mock_queue_service_class.return_value = mock_queue_service

        mock_query_service = AsyncMock()
        mock_query_service.list_indexes.return_value = []
        mock_query_service_class.return_value = mock_query_service

        # Create scheduler
        scheduler = SyncSchedulerService(mock_session_factory)

        # Perform sync
        await scheduler._perform_sync()  # noqa: SLF001

        # Verify no sync was attempted
        assert mock_query_service.list_indexes.called
        assert not mock_queue_service.enqueue_task.called


@pytest.mark.asyncio
async def test_sync_scheduler_handles_sync_failures(
    mock_session_factory: Callable[[], AsyncSession],
    dummy_indexes: list[Index],
) -> None:
    """Test that the sync scheduler handles failures gracefully."""
    with (
        patch(
            "kodit.application.services.sync_scheduler.QueueService"
        ) as mock_queue_service_class,
        patch(
            "kodit.application.services.sync_scheduler.IndexQueryService"
        ) as mock_query_service_class,
        patch("kodit.application.services.sync_scheduler.SqlAlchemyIndexRepository"),
    ):
        # Set up mocks with one failure
        mock_queue_service = AsyncMock()
        mock_queue_service.enqueue_task.side_effect = [
            None,  # First index succeeds
            Exception("Enqueue failed"),  # Second index fails
        ]
        mock_queue_service_class.return_value = mock_queue_service

        mock_query_service = AsyncMock()
        mock_query_service.list_indexes.return_value = dummy_indexes
        mock_query_service_class.return_value = mock_query_service

        # Create scheduler
        scheduler = SyncSchedulerService(mock_session_factory)

        # Perform sync - should raise exception since enqueue fails
        with pytest.raises(Exception, match="Enqueue failed"):
            await scheduler._perform_sync()  # noqa: SLF001

        # Verify first index was attempted
        assert mock_queue_service.enqueue_task.call_count == 2


@pytest.mark.asyncio
async def test_sync_scheduler_periodicity() -> None:
    """Test that the sync scheduler runs periodically at the specified interval."""
    mock_session_factory = AsyncMock()

    # Track sync operations
    sync_count = 0
    sync_times = []

    async def mock_perform_sync() -> None:
        nonlocal sync_count
        sync_count += 1
        sync_times.append(asyncio.get_event_loop().time())
        # Simulate a quick sync
        await asyncio.sleep(0.01)

    # Create scheduler with a very short interval for testing
    scheduler = SyncSchedulerService(mock_session_factory)

    # Patch the _perform_sync method
    with patch.object(scheduler, "_perform_sync", mock_perform_sync):
        # Start the sync task
        sync_task = asyncio.create_task(
            scheduler._sync_loop(interval_seconds=0.06)  # 0.06 seconds  # noqa: SLF001
        )

        # Let it run for a short time
        await asyncio.sleep(0.15)  # Should allow ~2-3 syncs

        # Stop the scheduler
        scheduler._shutdown_event.set()  # noqa: SLF001
        await sync_task

        # Verify multiple syncs occurred
        assert sync_count >= 2

        # Verify timing between syncs (with some tolerance)
        if len(sync_times) >= 2:
            for i in range(1, len(sync_times)):
                interval = sync_times[i] - sync_times[i - 1]
                # Should be around 0.06 seconds with some tolerance
                assert 0.04 < interval < 0.08


@pytest.mark.asyncio
async def test_sync_scheduler_start_stop() -> None:
    """Test starting and stopping the sync scheduler."""
    mock_session_factory = AsyncMock()

    sync_performed = False

    async def mock_perform_sync() -> None:
        nonlocal sync_performed
        sync_performed = True
        # Wait a bit to simulate work
        await asyncio.sleep(0.01)

    scheduler = SyncSchedulerService(mock_session_factory)

    with patch.object(scheduler, "_perform_sync", mock_perform_sync):
        # Start the scheduler in the background
        scheduler.start_periodic_sync(interval_seconds=0.06)

        # Give it time to perform at least one sync
        await asyncio.sleep(0.05)

        # Stop the scheduler
        await scheduler.stop_periodic_sync()

        # Wait for the sync task to complete (it may be cancelled)
        if scheduler._sync_task:  # noqa: SLF001
            with contextlib.suppress(asyncio.CancelledError):
                await scheduler._sync_task  # noqa: SLF001

        # Verify at least one sync was performed
        assert sync_performed

        # Verify the sync task is done
        assert scheduler._sync_task is not None  # noqa: SLF001
        assert scheduler._sync_task.done()  # noqa: SLF001


@pytest.mark.asyncio
async def test_sync_scheduler_handles_exceptions_in_sync_loop() -> None:
    """Test that exceptions in the sync loop don't crash the scheduler."""
    mock_session_factory = AsyncMock()

    exception_count = 0

    class TestExceptionError(Exception):
        """Test exception for sync scheduler tests."""

    async def mock_perform_sync() -> None:
        nonlocal exception_count
        exception_count += 1
        raise TestExceptionError("Test exception")

    scheduler = SyncSchedulerService(mock_session_factory)

    with patch.object(scheduler, "_perform_sync", mock_perform_sync):
        # Run the sync loop for a short time
        sync_task = asyncio.create_task(
            scheduler._sync_loop(interval_seconds=0.06)  # noqa: SLF001
        )

        # Let it run and fail a few times
        await asyncio.sleep(0.2)

        # Stop the scheduler
        scheduler._shutdown_event.set()  # noqa: SLF001
        await sync_task

        # Verify multiple exceptions were raised but the loop continued
        assert exception_count >= 2


@pytest.mark.asyncio
async def test_sync_scheduler_shutdown_during_sync() -> None:
    """Test that the scheduler can be shutdown while a sync is in progress."""
    mock_session_factory = AsyncMock()

    sync_started = asyncio.Event()
    sync_should_continue = asyncio.Event()

    async def mock_perform_sync() -> None:
        sync_started.set()
        # Wait for signal to continue
        await sync_should_continue.wait()

    scheduler = SyncSchedulerService(mock_session_factory)

    with patch.object(scheduler, "_perform_sync", mock_perform_sync):
        # Start the scheduler
        scheduler.start_periodic_sync(interval_seconds=1800)

        # Wait for sync to start
        await sync_started.wait()

        # Stop the scheduler while sync is in progress
        stop_task = asyncio.create_task(scheduler.stop_periodic_sync())

        # Give stop a moment to signal shutdown
        await asyncio.sleep(0.01)

        # Allow the sync to complete
        sync_should_continue.set()

        # Wait for stop task to complete
        await stop_task

        # Verify clean shutdown
        assert scheduler._shutdown_event.is_set()  # noqa: SLF001
        assert scheduler._sync_task is not None  # noqa: SLF001
        assert scheduler._sync_task.done()  # noqa: SLF001
