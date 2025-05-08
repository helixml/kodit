"""Index service for managing code indexes.

This module provides the IndexService class which handles the business logic for
creating, listing, and running code indexes. It orchestrates the interaction between the
file system, database operations (via IndexRepository), and provides a clean API for
index management.
"""

from datetime import datetime

import aiofiles
import pydantic
import structlog
from tqdm.asyncio import tqdm

from kodit.indexing.models import Snippet
from kodit.indexing.repository import IndexRepository
from kodit.sources.service import SourceService

# List of MIME types that are supported for indexing and snippet creation
MIME_WHITELIST = [
    "text/plain",
    "text/markdown",
    "text/x-python",
    "text/x-shellscript",
    "text/x-sql",
]


class IndexView(pydantic.BaseModel):
    """Data transfer object for index information.

    This model represents the public interface for index data, providing a clean
    view of index information without exposing internal implementation details.
    """

    id: int
    created_at: datetime
    updated_at: datetime | None = None
    source_uri: str | None = None
    num_snippets: int | None = None


class IndexService:
    """Service for managing code indexes.

    This service handles the business logic for creating, listing, and running code
    indexes. It coordinates between file system operations, database operations (via
    IndexRepository), and provides a clean API for index management.
    """

    def __init__(
        self, repository: IndexRepository, source_service: SourceService
    ) -> None:
        """Initialize the index service.

        Args:
            repository: The repository instance to use for database operations.
            source_service: The source service instance to use for source validation.

        """
        self.repository = repository
        self.source_service = source_service
        self.log = structlog.get_logger(__name__)

    async def create(self, source_id: int) -> IndexView:
        """Create a new index for a source.

        This method creates a new index for the specified source, after validating
        that the source exists and doesn't already have an index.

        Args:
            source_id: The ID of the source to create an index for.

        Returns:
            An Index object representing the newly created index.

        Raises:
            ValueError: If the source doesn't exist or already has an index.

        """
        # Check if the source exists
        source = await self.source_service.get(source_id)

        index = await self.repository.create(source.id)
        return IndexView(
            id=index.id,
            created_at=index.created_at,
        )

    async def list_indexes(self) -> list[IndexView]:
        """List all available indexes with their details.

        Returns:
            A list of Index objects containing information about each index,
            including file and snippet counts.

        """
        indexes = await self.repository.list_indexes()

        # Transform database results into DTOs
        return [
            IndexView(
                id=index.id,
                created_at=index.created_at,
                updated_at=index.updated_at,
                num_snippets=await self.repository.num_snippets_for_index(index.id),
            )
            for index in indexes
        ]

    async def run(self, index_id: int) -> None:
        """Run the indexing process for a specific index."""
        # Get and validate index
        index = await self.repository.get_by_id(index_id)
        if not index:
            msg = f"Index not found: {index_id}"
            raise ValueError(msg)

        # Create snippets for supported file types
        await self._create_snippets(index_id)

        # Update index timestamp
        await self.repository.update_index_timestamp(index)

    async def _create_snippets(
        self,
        index_id: int,
    ) -> None:
        """Create snippets for supported files.

        Args:
            index: The index to create snippets for.
            file_list: List of files to create snippets from.
            existing_snippets_set: Set of file IDs that already have snippets.

        """
        files = await self.repository.files_for_index(index_id)
        for file in tqdm(files, total=len(files)):
            # Skip unsupported file types
            if file.mime_type not in MIME_WHITELIST:
                self.log.debug("Skipping mime type", mime_type=file.mime_type)
                continue

            # Create snippet from file content
            async with aiofiles.open(file.cloned_path, "rb") as f:
                content = await f.read()
                snippet = Snippet(
                    index_id=index_id,
                    file_id=file.id,
                    content=content.decode("utf-8"),
                )
                await self.repository.add_snippet(snippet)
