"""Source service for managing code sources.

This module provides the SourceService class which handles the business logic for
creating and listing code sources. It orchestrates the interaction between the file
system, database operations (via SourceRepository), and provides a clean API for
source management.
"""

import shutil
from datetime import datetime
from pathlib import Path

import pydantic
import structlog
from uritools import isuri, urisplit

from kodit.sources.repository import SourceRepository

CLONE_DIR = Path("./kodit/clones")


class SourceView(pydantic.BaseModel):
    """View model for displaying source information.

    This model provides a clean interface for displaying source information,
    containing only the essential fields needed for presentation.

    Attributes:
        id: The unique identifier for the source.
        uri: The URI or path of the source.
        created_at: Timestamp when the source was created.

    """

    id: int
    uri: str
    cloned_path: Path
    created_at: datetime


class SourceService:
    """Service for managing code sources.

    This service handles the business logic for creating and listing code sources.
    It coordinates between file system operations, database operations (via
    SourceRepository), and provides a clean API for source management.
    """

    def __init__(self, repository: SourceRepository) -> None:
        """Initialize the source service.

        Args:
            repository: The repository instance to use for database operations.

        """
        self.repository = repository
        self.log = structlog.get_logger(__name__)

    async def create(self, uri_or_path_like: str) -> None:
        """Create a new source from a URI.

        Args:
            uri: The URI of the source to create. Can be a git-like URI or a local
                directory.

        Raises:
            ValueError: If the source type is not supported or if the folder doesn't
                exist.

        """
        if Path(uri_or_path_like).is_dir():
            return await self._create_folder_source(Path(uri_or_path_like))
        if isuri(uri_or_path_like):
            parsed = urisplit(uri_or_path_like)
            if parsed.scheme == "file":
                return await self._create_folder_source(Path(parsed.path))

        msg = f"Unsupported source type: {uri_or_path_like}"
        raise ValueError(msg)

    async def _create_folder_source(self, directory: Path) -> None:
        """Create a folder source.

        Args:
            directory: The path to the local directory.

        Raises:
            ValueError: If the folder doesn't exist or is already added.

        """
        # Check if the folder exists
        if not directory.exists():
            msg = f"Folder does not exist: {directory}"
            raise ValueError(msg)

        # Check if the folder is already added
        if await self.repository.get_source_by_uri(directory.as_uri()):
            msg = f"Directory already added: {directory}"
            raise ValueError(msg)

        # Clone into a local directory
        clone_path = CLONE_DIR / directory.as_posix().replace("/", "_")
        clone_path.mkdir(parents=True, exist_ok=True)

        # Copy all files recursively, preserving directory structure, ignoring hidden files
        shutil.copytree(
            directory,
            clone_path,
            ignore=shutil.ignore_patterns(".*"),
            dirs_exist_ok=True,
        )

        await self.repository.create_source(
            uri=directory.as_uri(),
            cloned_path=clone_path,
        )

    async def list_sources(self) -> list[SourceView]:
        """List all available sources.

        Returns:
            A list of SourceView objects containing information about each source.

        """
        sources = await self.repository.list_sources()
        return [
            SourceView(
                id=source.id,
                uri=source.uri,
                cloned_path=source.cloned_path,
                created_at=source.created_at,
            )
            for source in sources
        ]
