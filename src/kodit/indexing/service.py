"""Index service for managing code indexes.

This module provides the IndexService class which handles the business logic for
creating, listing, and running code indexes. It orchestrates the interaction between the
file system, database operations (via IndexRepository), and provides a clean API for
index management.
"""

import mimetypes
from datetime import datetime
from hashlib import sha256
from pathlib import Path

import aiofiles
import pydantic
import structlog
from tqdm.asyncio import tqdm

from kodit.indexing.models import File, Snippet
from kodit.indexing.models import Index as IndexModel
from kodit.indexing.repository import IndexRepository

# List of MIME types that are supported for indexing and snippet creation
MIME_WHITELIST = [
    "text/plain",
    "text/markdown",
    "text/x-python",
    "text/x-shellscript",
    "text/x-sql",
]


class Index(pydantic.BaseModel):
    """Data transfer object for index information.

    This model represents the public interface for index data, providing a clean
    view of index information without exposing internal implementation details.
    """

    id: int
    created_at: datetime
    updated_at: datetime | None = None
    source_uri: str | None = None
    num_files: int | None = None
    num_snippets: int | None = None


class IndexService:
    """Service for managing code indexes.

    This service handles the business logic for creating, listing, and running code
    indexes. It coordinates between file system operations, database operations (via
    IndexRepository), and provides a clean API for index management.
    """

    def __init__(self, repository: IndexRepository) -> None:
        """Initialize the index service.

        Args:
            repository: The repository instance to use for database operations.

        """
        self.repository = repository
        self.log = structlog.get_logger(__name__)

    async def create(self, source_id: int) -> Index:
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
        # Validate source existence
        source_details = await self.repository.get_source_details(source_id)
        if not source_details:
            msg = f"Source not found, please create it first: {source_id}"
            raise ValueError(msg)

        # Check for existing index
        existing_index = await self.repository.get_by_id(source_id)
        if existing_index:
            msg = f"Index already exists on this source: {source_id}"
            raise ValueError(msg)

        # Create and return the new index
        index = await self.repository.create(source_id)
        return Index(
            id=index.id, created_at=index.created_at, source_id=index.source_id
        )

    async def list_indexes(self) -> list[Index]:
        """List all available indexes with their details.

        Returns:
            A list of Index objects containing information about each index,
            including file and snippet counts.

        """
        rows = await self.repository.list_with_details()

        # Transform database results into DTOs
        return [
            Index(
                id=index.id,
                created_at=index.created_at,
                updated_at=index.updated_at,
                source_uri=(
                    git_source.uri
                    if git_source
                    else folder_source.path
                    if folder_source
                    else None
                ),
                num_files=file_count,
                num_snippets=snippet_count,
            )
            for (
                index,
                source,
                git_source,
                folder_source,
                file_count,
                snippet_count,
            ) in rows
        ]

    async def _process_file(
        self,
        file_path: Path,
        index: IndexModel,
        existing_files_set: set[str],
    ) -> None:
        """Process a single file for indexing.

        Args:
            file_path: The path to the file to process.
            index: The index to add the file to.
            existing_files_set: Set of already indexed file hashes.

        """
        if not file_path.is_file():
            return

        async with aiofiles.open(file_path, "rb") as f:
            content = await f.read()
            mime_type = mimetypes.guess_type(file_path)
            sha = sha256(content).hexdigest()

            # Skip if file already indexed
            if sha in existing_files_set:
                self.log.debug("File already exists", file_path=file_path)
                return

            # Create file record
            file = File(
                index_id=index.id,
                source_id=index.source_id,
                mime_type=mime_type[0]
                if mime_type and mime_type[0]
                else "application/octet-stream",
                path=str(file_path),
                sha256=sha,
                size_bytes=len(content),
            )

            self.log.debug("Adding file", file=file)
            await self.repository.add_file(file)

    async def _create_snippets(
        self,
        index: IndexModel,
        file_list: list[File],
        existing_snippets_set: set[int],
    ) -> None:
        """Create snippets for supported files.

        Args:
            index: The index to create snippets for.
            file_list: List of files to create snippets from.
            existing_snippets_set: Set of file IDs that already have snippets.

        """
        for file in tqdm(file_list, total=len(file_list)):
            # Skip unsupported file types
            if file.mime_type not in MIME_WHITELIST:
                self.log.debug("Skipping mime type", mime_type=file.mime_type)
                continue

            # Skip if snippet already exists
            if file.id in existing_snippets_set:
                self.log.debug("Snippet already exists", file_id=file.id)
                continue

            # Create snippet from file content
            async with aiofiles.open(file.path, "rb") as f:
                content = await f.read()
                snippet = Snippet(
                    index_id=index.id,
                    file_id=file.id,
                    content=content,
                )
                await self.repository.add_snippet(snippet)

    async def run(self, index_id: int) -> None:
        """Run the indexing process for a specific index.

        This method performs the actual indexing process, which includes:
        1. Scanning the source directory for files
        2. Creating file records for new files
        3. Creating snippets for supported file types
        4. Updating the index timestamp

        Args:
            index_id: The ID of the index to run.

        Raises:
            ValueError: If the index or its source doesn't exist.
            NotImplementedError: If the source is a git repository (not yet supported).
            TypeError: If the source type is not supported.

        """
        # Get and validate index
        index = await self.repository.get_by_id(index_id)
        if not index:
            msg = f"Index not found: {index_id}"
            raise ValueError(msg)

        # Get and validate source details
        source_details = await self.repository.get_source_details(index.source_id)
        if not source_details:
            msg = f"Source not found: {index.source_id}"
            raise ValueError(msg)

        source, git_source, folder_source = source_details

        # Get existing files to avoid duplicates
        existing_files_set = await self.repository.get_existing_files(index.source_id)

        if git_source:
            msg = "Git source indexing is not implemented yet"
            raise NotImplementedError(msg)
        if folder_source:
            # Count total files for progress bar
            file_count = sum(
                1 for _ in Path(folder_source.path).rglob("*") if _.is_file()
            )

            # Process each file in the source directory
            for file_path in tqdm(
                Path(folder_source.path).rglob("*"), total=file_count
            ):
                await self._process_file(file_path, index, existing_files_set)

            # Get all files for snippet creation
            files = await self.repository.get_files_by_source(index.source_id)
            existing_snippets_set = await self.repository.get_existing_snippets(
                index.id
            )

            # Create snippets for supported file types
            await self._create_snippets(index, files, existing_snippets_set)

            # Update index timestamp
            await self.repository.update_index_timestamp(index)
        else:
            msg = f"Unsupported source type: {type(source)}"
            raise TypeError(msg)
