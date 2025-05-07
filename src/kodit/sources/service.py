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
    """Source model."""

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

    async def create(self, uri: str) -> SourceView:
        """Create a new source from a URI.

        Args:
            uri: The URI of the source to create. Can be a git-like URI or a local
                directory.

        Returns:
            The newly created Source instance.

        Raises:
            ValueError: If the source type is not supported or if the folder doesn't
                exist.

        """
        if Path(uri).is_dir():
            return await self._create_folder_source(uri)
        msg = (
            f"Unsupported source type: {uri}. "
            "Please pass a git-like URI or a local directory."
        )
        raise ValueError(msg)

    async def _create_git_source(self, uri: str) -> SourceView:
        """Create a git source.

        Args:
            uri: The git repository URI.

        Returns:
            A Source object representing the newly created git source.

        """
        source = await self.repository.create_git_source(uri)
        return SourceView(
            id=source.id,
            uri=uri,
            created_at=source.created_at,
        )

    async def _create_folder_source(self, uri: str) -> SourceView:
        """Create a folder source.

        Args:
            uri: The path to the local directory.

        Returns:
            A Source object representing the newly created folder source.

        Raises:
            ValueError: If the folder doesn't exist or is already added.

        """
        # Expand uri into a full path
        uri = Path(uri).expanduser().resolve()

        # Check if the folder exists
        if not uri.exists():
            msg = f"Folder does not exist: {uri}"
            raise ValueError(msg)

        source = await self.repository.create_folder_source(str(uri))
        return SourceView(
            id=source.id,
            uri=str(uri),
            created_at=source.created_at,
        )

    async def list(self) -> list[SourceView]:
        """List all available sources.

        Returns:
            A list of Source objects containing information about each source.

        """
        sources = await self.repository.list()
        return [
            SourceView(
                id=source.id,
                uri=source.uri,
                created_at=source.created_at,
            )
            for source in sources
        ]
