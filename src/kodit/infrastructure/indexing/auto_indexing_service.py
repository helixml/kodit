"""Service for automatically indexing configured sources."""

import asyncio
from collections.abc import Callable
from contextlib import suppress

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.code_indexing_factory import (
    create_code_indexing_application_service,
)
from kodit.config import AppContext


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

        auto_sources = [source.uri for source in self.app_context.auto_indexing.sources]
        self.log.info("Starting background indexing", num_sources=len(auto_sources))
        self._indexing_task = asyncio.create_task(self._index_sources(auto_sources))

    async def _index_sources(self, sources: list[str]) -> None:
        """Index all configured sources in the background."""
        async with self.session_factory() as session:
            service = create_code_indexing_application_service(
                app_context=self.app_context,
                session=session,
            )

            for source in sources:
                try:
                    self.log.info("Auto-indexing source", source=source)

                    # Create index
                    index = await service.create_index_from_uri(source)

                    # Run indexing (without progress callback for background mode)
                    await service.run_index(index, progress_callback=None)

                    self.log.info("Successfully auto-indexed source", source=source)

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
