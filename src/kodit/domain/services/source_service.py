"""Source service rewritten to work directly with AsyncSession."""

from collections.abc import Callable
from pathlib import Path

import structlog
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from kodit.domain.entities import Source
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.repositories import SourceRepository
from kodit.infrastructure.cloning.folder.factory import FolderSourceFactory
from kodit.infrastructure.cloning.folder.working_copy import FolderWorkingCopyProvider
from kodit.infrastructure.cloning.git.factory import (
    GitSourceFactory,
    GitWorkingCopyProvider,
)
from kodit.infrastructure.git.git_utils import is_valid_clone_target
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

    async def create(
        self, uri_or_path_like: str, progress_callback: ProgressCallback | None = None
    ) -> Source:
        """Create a source."""
        async with self._session_factory() as session:
            repo = SqlAlchemySourceRepository(session)
            git_factory, folder_factory = self._build_factories(repo, session)

            if is_valid_clone_target(uri_or_path_like):
                source = await git_factory.create(uri_or_path_like, progress_callback)
            elif Path(uri_or_path_like).is_dir():
                source = await folder_factory.create(
                    uri_or_path_like, progress_callback
                )
            else:
                raise ValueError(f"Unsupported source: {uri_or_path_like}")

            # Factories handle their own commits now
            return source

    def _build_factories(
        self, repository: SourceRepository, session: AsyncSession
    ) -> tuple[GitSourceFactory, FolderSourceFactory]:
        # Git-specific collaborators
        git_wc = GitWorkingCopyProvider(self.clone_dir)
        git_factory = GitSourceFactory(
            repository=repository,
            working_copy=git_wc,
            session=session,
        )

        # Folder-specific collaborators
        fold_wc = FolderWorkingCopyProvider(self.clone_dir)
        folder_factory = FolderSourceFactory(
            repository=repository,
            working_copy=fold_wc,
            session=session,
        )

        return git_factory, folder_factory
