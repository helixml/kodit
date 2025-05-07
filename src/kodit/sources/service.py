"""Source service for managing code sources.

This module provides the SourceService class which handles the business logic for
creating and listing code sources. It orchestrates the interaction between the file
system, database operations (via SourceRepository), and provides a clean API for
source management.
"""

from datetime import datetime
from pathlib import Path

import pydantic
import structlog

from kodit.sources.repository import SourceRepository


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

    async def create(self, uri: str) -> None:
        """Create a new source from a URI.

        Args:
            uri: The URI of the source to create. Can be a git-like URI or a local
                directory.

        Raises:
            ValueError: If the source type is not supported or if the folder doesn't
                exist.

        """
        if Path(uri).is_dir():
            return await self._create_folder_source(uri)
        return await self._create_git_source(uri)

    async def _create_git_source(self, uri: str) -> None:
        """Create a git source.

        Args:
            uri: The git repository URI.

        Raises:
            ValueError: If the git repository cannot be created.

        """
        await self.repository.create_git_source(uri)

    async def _create_folder_source(self, uri: str) -> None:
        """Create a folder source.

        Args:
            uri: The path to the local directory.

        Raises:
            ValueError: If the folder doesn't exist or is already added.

        """
        # Expand uri into a full path
        uri = Path(uri).expanduser().resolve()

        # Check if the folder exists
        if not uri.exists():
            msg = f"Folder does not exist: {uri}"
            raise ValueError(msg)

        await self.repository.create_folder_source(str(uri))

    async def list_sources(self) -> list[SourceView]:
        """List all available sources.

        Returns:
            A list of SourceView objects containing information about each source.

        """
        sources = await self.repository.list_sources()
        return [
            SourceView(
                id=source.id,
                uri=git_source.uri if git_source else folder_source.path,
                created_at=source.created_at,
            )
            for (source, git_source, folder_source) in sources
        ]
