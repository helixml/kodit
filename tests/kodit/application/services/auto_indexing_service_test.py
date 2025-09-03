"""Tests for the auto-indexing service."""

import asyncio
from collections.abc import Callable
from types import TracebackType
from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.auto_indexing_service import AutoIndexingService
from kodit.config import AppContext, AutoIndexingConfig, AutoIndexingSource


@pytest.mark.filterwarnings("ignore::RuntimeWarning")
class TestAutoIndexingService:
    """Test the AutoIndexingService."""

    @pytest.fixture
    def mock_sources(self) -> list[AutoIndexingSource]:
        """Create mock auto-indexing sources."""
        return [
            AutoIndexingSource(uri="https://github.com/test/repo1"),
            AutoIndexingSource(uri="https://github.com/test/repo2"),
            AutoIndexingSource(uri="/local/test/path"),
        ]

    @pytest.fixture
    def app_context_with_sources(
        self, mock_sources: list[AutoIndexingSource]
    ) -> AppContext:
        """Create app context with auto-indexing sources."""
        return AppContext(auto_indexing=AutoIndexingConfig(sources=mock_sources))

    @pytest.fixture
    def mock_session_factory(self) -> Callable[[], Any]:
        """Create a mock session factory."""

        # Create a mock that is an async context manager
        class DummyAsyncContextManager:
            async def __aenter__(self) -> AsyncMock:
                return AsyncMock(spec=AsyncSession)

            async def __aexit__(
                self,
                exc_type: type[BaseException] | None,
                exc: BaseException | None,
                tb: TracebackType | None,
            ) -> None:
                return None

        def factory() -> DummyAsyncContextManager:
            return DummyAsyncContextManager()

        return factory

    @pytest.fixture
    def auto_indexing_service(
        self,
        app_context_with_sources: AppContext,
        mock_session_factory: Callable[[], Any],
    ) -> AutoIndexingService:
        """Create an AutoIndexingService instance."""
        return AutoIndexingService(
            app_context=app_context_with_sources,
            session_factory=mock_session_factory,
        )

    @pytest.mark.asyncio
    async def test_start_background_indexing_enabled(
        self, auto_indexing_service: AutoIndexingService
    ) -> None:
        """Test starting background indexing when enabled."""
        # Mock the services
        with patch(
            "kodit.application.services.auto_indexing_service.create_code_indexing_application_service"
        ) as mock_create_service:
            mock_service = AsyncMock()
            mock_create_service.return_value = mock_service

            # Mock the index creation and indexing
            mock_index = MagicMock()
            mock_index.id = "test-index-id"
            mock_service.create_index_from_uri.return_value = mock_index

            # Start background indexing
            await auto_indexing_service.start_background_indexing()

            # Wait a bit for the task to start
            await asyncio.sleep(0.1)

            # Verify the task was created
            assert auto_indexing_service._indexing_task is not None  # noqa: SLF001
            # Optionally, check that the task completed successfully
            assert auto_indexing_service._indexing_task.done()  # noqa: SLF001
            assert auto_indexing_service._indexing_task.exception() is None  # noqa: SLF001

            # Stop the service
            await auto_indexing_service.stop()

    @pytest.mark.asyncio
    async def test_start_background_indexing_disabled(
        self, mock_session_factory: Callable[[], Any]
    ) -> None:
        """Test starting background indexing when disabled."""
        app_context = AppContext(auto_indexing=AutoIndexingConfig(sources=[]))
        service = AutoIndexingService(
            app_context=app_context,
            session_factory=mock_session_factory,
        )

        await service.start_background_indexing()

        # Should not create a task when disabled
        assert service._indexing_task is None  # noqa: SLF001

    @pytest.mark.asyncio
    async def test_index_sources_success(
        self, auto_indexing_service: AutoIndexingService
    ) -> None:
        """Test successful indexing of sources."""
        with (
            patch(
                "kodit.application.services.auto_indexing_service.create_code_indexing_application_service"
            ) as mock_create_service,
            patch(
                "kodit.application.services.auto_indexing_service.QueueService"
            ) as mock_queue_service_class,
        ):
            mock_service = AsyncMock()
            mock_create_service.return_value = mock_service

            mock_queue_service = AsyncMock()
            mock_queue_service_class.return_value = mock_queue_service

            # Mock the index creation
            mock_index = MagicMock()
            mock_index.id = 1
            mock_service.does_index_exist.return_value = False
            mock_service.create_index_from_uri.return_value = mock_index

            # Test indexing sources directly
            sources = ["https://github.com/test/repo1", "https://github.com/test/repo2"]
            await auto_indexing_service._index_sources(sources)  # noqa: SLF001

            # Verify both sources were processed
            assert mock_service.create_index_from_uri.call_count == 2
            # Tasks should be enqueued, not run directly
            assert mock_queue_service.enqueue_task.call_count == 2

    @pytest.mark.asyncio
    async def test_index_sources_with_failure(
        self, auto_indexing_service: AutoIndexingService
    ) -> None:
        """Test indexing sources with one failure."""
        with (
            patch(
                "kodit.application.services.auto_indexing_service.create_code_indexing_application_service"
            ) as mock_create_service,
            patch(
                "kodit.application.services.auto_indexing_service.QueueService"
            ) as mock_queue_service_class,
        ):
            mock_service = AsyncMock()
            mock_create_service.return_value = mock_service

            mock_queue_service = AsyncMock()
            mock_queue_service_class.return_value = mock_queue_service

            # Mock the first source to succeed, second to fail
            mock_index = MagicMock()
            mock_index.id = 1
            mock_service.does_index_exist.return_value = False
            mock_service.create_index_from_uri.side_effect = [
                mock_index,
                Exception("Test error"),
            ]

            # Test indexing sources directly
            sources = ["https://github.com/test/repo1", "https://github.com/test/repo2"]
            await auto_indexing_service._index_sources(sources)  # noqa: SLF001

            # Verify both sources were attempted
            assert mock_service.create_index_from_uri.call_count == 2
            # First source should have been enqueued (second failed during creation)
            assert mock_queue_service.enqueue_task.call_count == 1

    @pytest.mark.asyncio
    async def test_stop_service(
        self, auto_indexing_service: AutoIndexingService
    ) -> None:
        """Test stopping the auto-indexing service."""

        # Create a real task that we can cancel
        async def dummy_task() -> None:
            await asyncio.sleep(1)

        task = asyncio.create_task(dummy_task())
        auto_indexing_service._indexing_task = task  # noqa: SLF001

        await auto_indexing_service.stop()

        # Task should be cancelled
        assert task.cancelled()

    @pytest.mark.asyncio
    async def test_stop_service_no_task(
        self, auto_indexing_service: AutoIndexingService
    ) -> None:
        """Test stopping the service when no task exists."""
        auto_indexing_service._indexing_task = None  # noqa: SLF001

        # Should not raise an exception
        await auto_indexing_service.stop()
