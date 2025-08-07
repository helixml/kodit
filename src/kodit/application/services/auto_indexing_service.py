"""Service for automatically indexing configured sources."""

import asyncio
import warnings
from collections.abc import Callable
from contextlib import suppress

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.code_indexing_factory import (
    create_code_indexing_application_service,
)
from kodit.application.services.queue_service import QueueService
from kodit.config import AppContext
from kodit.domain.entities import Task
from kodit.domain.value_objects import QueuePriority


class AutoIndexingService:
    """Service for automatically indexing configured sources."""

    def __init__(
        self,
        app_context: AppContext,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Initialize the auto-indexing service."""
        self.app_context = app_context
        self.session_factory = session_factory
        self.log = structlog.get_logger(__name__)
        self._indexing_task: asyncio.Task | None = None

    async def start_background_indexing(self) -> None:
        """Start background indexing of configured sources."""
        if (
            not self.app_context.auto_indexing
            or len(self.app_context.auto_indexing.sources) == 0
        ):
            self.log.info("Auto-indexing is disabled (no sources configured)")
            return

        warnings.warn(
            "Auto-indexing is deprecated and will be removed in a future version, please use the API to index sources.",  # noqa: E501
            DeprecationWarning,
            stacklevel=2,
        )

        auto_sources = [source.uri for source in self.app_context.auto_indexing.sources]
        self.log.info("Starting background indexing", num_sources=len(auto_sources))
        self._indexing_task = asyncio.create_task(self._index_sources(auto_sources))

    async def _index_sources(self, sources: list[str]) -> None:
        """Index all configured sources in the background."""
        async with self.session_factory() as session:
            queue_service = QueueService(session=session)
            service = create_code_indexing_application_service(
                app_context=self.app_context,
                session=session,
            )

            for source in sources:
                try:
                    # Only auto-index a source if it is new
                    if await service.does_index_exist(source):
                        self.log.info("Index already exists, skipping", source=source)
                        continue

                    self.log.info("Adding auto-indexing task to queue", source=source)

                    # Create index
                    index = await service.create_index_from_uri(source)

                    await queue_service.enqueue_task(
                        Task.create_index_update_task(
                            index.id, QueuePriority.BACKGROUND
                        )
                    )

                except Exception as exc:
                    self.log.exception(
                        "Failed to auto-index source", source=source, error=str(exc)
                    )
                    # Continue with other sources even if one fails

    async def stop(self) -> None:
        """Stop background indexing."""
        if self._indexing_task:
            self._indexing_task.cancel()
            with suppress(asyncio.CancelledError):
                await self._indexing_task
