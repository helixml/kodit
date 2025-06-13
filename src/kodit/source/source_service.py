"""Source service for managing code sources.

This module provides the SourceService class which handles the business logic for
creating and listing code sources. It orchestrates the interaction between the file
system, database operations (via SourceRepository), and provides a clean API for
source management.
"""

import mimetypes
import shutil
import tempfile
from datetime import UTC, datetime
from hashlib import sha256
from pathlib import Path

import aiofiles
import git
import pydantic
import structlog
from tqdm import tqdm

from kodit.source.git import is_valid_clone_target
from kodit.source.ignore import IgnorePatterns
from kodit.source.source_models import (
    Author,
    File,
    Source,
    SourceType,
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
            return await self._create_git_source(uri_or_path_like)

        # Otherwise just treat it as a directory
        if Path(uri_or_path_like).is_dir():
            return await self._create_folder_source(Path(uri_or_path_like))

        msg = f"Unsupported source: {uri_or_path_like}"
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
                Source(
                    uri=directory.as_uri(),
                    cloned_path=str(clone_path),
                    source_type=SourceType.FOLDER,
                ),
            )

            # Add all files to the source
            # Count total files for progress bar
            file_count = sum(1 for _ in clone_path.rglob("*") if _.is_file())

            # Process each file in the source directory
            for path in tqdm(clone_path.rglob("*"), total=file_count, leave=False):
                await self._process_file(source, path.absolute())

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
        self.log.debug("Normalising git uri", uri=uri)
        with tempfile.TemporaryDirectory() as temp_dir:
            git.Repo.clone_from(uri, temp_dir)
            remote = git.Repo(temp_dir).remote()
            uri = remote.url

        self.log.debug("Checking if source already exists", uri=uri)
        source = await self.repository.get_source_by_uri(uri)

        if source:
            self.log.info("Source already exists, reusing...", source_id=source.id)
        else:
            # Create a unique directory name for the clone
            clone_path = self.clone_dir / uri.replace("/", "_").replace(":", "_")
            clone_path.mkdir(parents=True, exist_ok=True)

            try:
                self.log.info("Cloning repository", uri=uri, clone_path=str(clone_path))
                git.Repo.clone_from(uri, clone_path)
            except git.GitCommandError as e:
                if "already exists and is not an empty directory" in str(e):
                    self.log.info("Repository already exists, reusing...", uri=uri)
                else:
                    msg = f"Failed to clone repository: {e}"
                    raise ValueError(msg) from e

            self.log.debug("Creating source", uri=uri, clone_path=str(clone_path))
            source = await self.repository.create_source(
                Source(
                    uri=uri,
                    cloned_path=str(clone_path),
                    source_type=SourceType.GIT,
                ),
            )

            # Get the ignore patterns for this source
            ignore_patterns = IgnorePatterns(clone_path)

            # Get all files that are not ignored
            files = [
                f for f in clone_path.rglob("*") if not ignore_patterns.should_ignore(f)
            ]

            # Process each file in the source directory
            self.log.info("Inspecting files", source_id=source.id, num_files=len(files))
            for path in tqdm(files, total=len(files), leave=False):
                await self._process_file(source, path.absolute())

        return SourceView(
            id=source.id,
            uri=source.uri,
            cloned_path=Path(source.cloned_path),
            created_at=source.created_at,
            num_files=await self.repository.num_files_for_source(source.id),
        )

    async def _process_file(
        self,
        source: Source,
        cloned_file: Path,
    ) -> None:
        """Process a single file for indexing."""
        if not cloned_file.is_file():
            return

        # If this file exists in a git repository, pull out the file's metadata
        authors: list[Author] = []
        first_modified_at: datetime | None = None
        last_modified_at: datetime | None = None
        if source.type == SourceType.GIT:
            # Get the git repository
            git_repo = git.Repo(source.cloned_path)

            # Get the last commit that touched this file
            commits = list(
                git_repo.iter_commits(
                    paths=str(cloned_file),
                    all=True,
                )
            )
            if len(commits) > 0:
                last_modified_at = commits[0].committed_datetime
                first_modified_at = commits[-1].committed_datetime

            # Get the file's blame
            blames = git_repo.blame("HEAD", str(cloned_file))

            # Extract the blame's authors
            actors = [
                commit.author
                for blame in blames or []
                for commit in blame
                if isinstance(commit, git.Commit)
            ]

            # Get or create the authors in the database
            for actor in actors:
                if actor.name or actor.email:
                    author = await self.repository.get_or_create_author(
                        actor.name or "", actor.email or ""
                    )
                    authors.append(author)

        # Create the file record
        async with aiofiles.open(cloned_file, "rb") as f:
            content = await f.read()
            mime_type = mimetypes.guess_type(cloned_file)
            sha = sha256(content).hexdigest()

            # Create file record
            file = File(
                created_at=first_modified_at or datetime.now(UTC),
                updated_at=last_modified_at or datetime.now(UTC),
                source_id=source.id,
                cloned_path=str(cloned_file),
                mime_type=mime_type[0]
                if mime_type and mime_type[0]
                else "application/octet-stream",
                uri=cloned_file.as_uri(),
                sha256=sha,
                size_bytes=len(content),
            )

            await self.repository.create_file(file)

            # Create mapping of authors to the file
            for author in authors:
                await self.repository.get_or_create_author_file_mapping(
                    author_id=author.id, file_id=file.id
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
