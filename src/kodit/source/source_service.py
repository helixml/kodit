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

from kodit.source.git import is_valid_clone_target
from kodit.source.source_factories import (
    FolderFileMetadataExtractor,
    FolderSourceFactory,
    FolderWorkingCopyProvider,
    GitAuthorExtractor,
    GitFileMetadataExtractor,
    GitSourceFactory,
    GitWorkingCopyProvider,
    NoOpAuthorExtractor,
)
from kodit.source.source_repository import SourceRepository


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
    num_files: int


class SourceService:
    """Service for managing code sources.

    This service handles the business logic for creating and listing code sources.
    It coordinates between file system operations, database operations (via
    SourceRepository), and provides a clean API for source management.
    """

    def __init__(self, clone_dir: Path, repository: SourceRepository) -> None:
        """Initialize the source service.

        Args:
            repository: The repository instance to use for database operations.

        """
        self.clone_dir = clone_dir
        self.repository = repository
        self.log = structlog.get_logger(__name__)

        # Initialize factories
        self._setup_factories()

    def _setup_factories(self) -> None:
        # Git-specific dependencies
        git_working_copy = GitWorkingCopyProvider(self.clone_dir)
        git_metadata_extractor = GitFileMetadataExtractor()
        git_author_extractor = GitAuthorExtractor(self.repository)
        self.git_factory = GitSourceFactory(
            working_copy=git_working_copy,
            metadata_extractor=git_metadata_extractor,
            author_extractor=git_author_extractor,
            repository=self.repository,
        )

        # Folder-specific dependencies
        folder_working_copy = FolderWorkingCopyProvider(self.clone_dir)
        folder_metadata_extractor = FolderFileMetadataExtractor()
        no_op_author_extractor = NoOpAuthorExtractor()
        self.folder_factory = FolderSourceFactory(
            working_copy=folder_working_copy,
            metadata_extractor=folder_metadata_extractor,
            author_extractor=no_op_author_extractor,
            repository=self.repository,
        )

    async def get(self, source_id: int) -> SourceView:
        """Get a source by ID.

        Args:
            source_id: The ID of the source to get.

        """
        source = await self.repository.get_source_by_id(source_id)
        if not source:
            msg = f"Source not found: {source_id}"
            raise ValueError(msg)
        return SourceView(
            id=source.id,
            uri=source.uri,
            cloned_path=Path(source.cloned_path),
            created_at=source.created_at,
            num_files=await self.repository.num_files_for_source(source.id),
        )

    async def create(self, uri_or_path_like: str) -> SourceView:
        """Create a new source from a URI or path."""
        # If it's possible to clone it, then do so
        if is_valid_clone_target(uri_or_path_like):
            source = await self.git_factory.create(uri_or_path_like)
        # Otherwise just treat it as a directory
        elif Path(uri_or_path_like).is_dir():
            source = await self.folder_factory.create(uri_or_path_like)
        else:
            msg = f"Unsupported source: {uri_or_path_like}"
            raise ValueError(msg)

        return SourceView(
            id=source.id,
            uri=source.uri,
            cloned_path=Path(source.cloned_path),
            created_at=source.created_at,
            num_files=await self.repository.num_files_for_source(source.id),
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
                cloned_path=Path(source.cloned_path),
                created_at=source.created_at,
                num_files=await self.repository.num_files_for_source(source.id),
            )
            for source in sources
        ]
