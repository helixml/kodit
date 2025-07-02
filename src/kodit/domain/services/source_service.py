"""Source service rewritten to work directly with AsyncSession."""

from collections.abc import Callable
from pathlib import Path

import structlog
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from kodit.domain.entities import Source
from kodit.infrastructure.sqlalchemy.repository import SqlAlchemySourceRepository


class SourceService:
    """Source service."""

    def __init__(
        self,
        clone_dir: Path,
        session_factory: async_sessionmaker[AsyncSession] | Callable[[], AsyncSession],
    ) -> None:
        """Initialize the source service."""
        self.clone_dir = clone_dir
        self._session_factory = session_factory
        self.log = structlog.get_logger(__name__)

    async def get(self, source_id: int) -> Source:
        """Get a source."""
        async with self._session_factory() as session:
            repo = SqlAlchemySourceRepository(session)

            source = await repo.get(source_id)
            if source is None:
                raise ValueError(f"Source not found: {source_id}")

            return source
