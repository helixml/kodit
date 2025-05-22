"""Source service for managing code sources.

This module provides the SourceService class which handles the business logic for
creating and listing code sources. It orchestrates the interaction between the file
system, database operations (via SourceRepository), and provides a clean API for
source management.
"""

import mimetypes
import shutil
from datetime import datetime
from hashlib import sha256
from pathlib import Path

import aiofiles
import git
import pydantic
import structlog
from tqdm import tqdm
from uritools import isuri, urisplit

from kodit.sources.models import File, Source
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
            if parsed.scheme in ("git", "http", "https") and parsed.path.endswith(
                ".git"
            ):
                return await self._create_git_source(uri_or_path_like)

            # Try adding a .git suffix, sometimes people just pass the url
            if not uri_or_path_like.endswith(".git"):
                uri_or_path_like = uri_or_path_like + ".git"
                try:
                    return await self._create_git_source(uri_or_path_like)
                except ValueError:
                    pass

        msg = f"Unsupported source type: {uri_or_path_like}"
        raise ValueError(msg)

    async def _create_folder_source(self, directory: Path) -> SourceView:
        """Create a folder source.

        Args:
            directory: The path to the local directory.

        Raises:
            ValueError: If the folder doesn't exist.
            SourceAlreadyExistsError: If the folder is already added.

        """
        # Resolve the directory to an absolute path
        directory = directory.expanduser().resolve()

        source = await self.repository.get_source_by_uri(directory.as_uri())
        if source:
            self.log.info("Source already exists, reusing...", source_id=source.id)
        else:
            # Check if the folder exists
            if not directory.exists():
                msg = f"Folder does not exist: {directory}"
                raise ValueError(msg)

            # Check if the folder is already added
            if await self.repository.get_source_by_uri(directory.as_uri()):
                msg = f"Directory already added: {directory}"
                raise ValueError(msg)

            # Clone into a local directory
            clone_path = self.clone_dir / directory.as_posix().replace("/", "_")
            clone_path.mkdir(parents=True, exist_ok=True)

            # Copy all files recursively, preserving directory structure, ignoring
            # hidden files
            shutil.copytree(
                directory,
                clone_path,
                ignore=shutil.ignore_patterns(".*"),
                dirs_exist_ok=True,
            )

            source = await self.repository.create_source(
                Source(uri=directory.as_uri(), cloned_path=str(clone_path)),
            )

            # Add all files to the source
            # Count total files for progress bar
            file_count = sum(1 for _ in clone_path.rglob("*") if _.is_file())

            # Process each file in the source directory
            for path in tqdm(clone_path.rglob("*"), total=file_count, leave=False):
                await self._process_file(source.id, path.absolute())

        return SourceView(
            id=source.id,
            uri=source.uri,
            cloned_path=Path(source.cloned_path),
            created_at=source.created_at,
            num_files=await self.repository.num_files_for_source(source.id),
        )

    async def _create_git_source(self, uri: str) -> SourceView:
        """Create a git source.

        Args:
            uri: The URI of the git repository.

        Raises:
            ValueError: If the repository cloning fails.

        """
        # Check if the repository is already added
        source = await self.repository.get_source_by_uri(uri)

        if source:
            self.log.info("Source already exists, reusing...", source_id=source.id)
        else:
            # Create a unique directory name for the clone
            clone_path = self.clone_dir / uri.replace("/", "_").replace(":", "_")
            clone_path.mkdir(parents=True, exist_ok=True)

            try:
                # Clone the repository
                git.Repo.clone_from(uri, clone_path)
            except git.GitCommandError as e:
                msg = f"Failed to clone repository: {e}"
                raise ValueError(msg) from e

            source = await self.repository.create_source(
                Source(uri=uri, cloned_path=str(clone_path)),
            )

            # Add all files to the source
            # Count total files for progress bar
            file_count = sum(1 for _ in clone_path.rglob("*") if _.is_file())

            # Process each file in the source directory
            for path in tqdm(clone_path.rglob("*"), total=file_count, leave=False):
                await self._process_file(source.id, path.absolute())

        return SourceView(
            id=source.id,
            uri=source.uri,
            cloned_path=Path(source.cloned_path),
            created_at=source.created_at,
            num_files=await self.repository.num_files_for_source(source.id),
        )

    async def _process_file(
        self,
        source_id: int,
        cloned_path: Path,
    ) -> None:
        """Process a single file for indexing."""
        if not cloned_path.is_file():
            return

        async with aiofiles.open(cloned_path, "rb") as f:
            content = await f.read()
            mime_type = mimetypes.guess_type(cloned_path)
            sha = sha256(content).hexdigest()

            # Create file record
            file = File(
                source_id=source_id,
                cloned_path=cloned_path.as_posix(),
                mime_type=mime_type[0]
                if mime_type and mime_type[0]
                else "application/octet-stream",
                uri=cloned_path.as_uri(),
                sha256=sha,
                size_bytes=len(content),
            )

            await self.repository.create_file(file)

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
