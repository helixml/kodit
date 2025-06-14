"""Source factories for creating different types of sources.

This module provides factory classes for creating sources, improving cohesion by
separating the concerns of different source types.
"""

import mimetypes
import shutil
import tempfile
from abc import ABC, abstractmethod
from datetime import UTC, datetime
from hashlib import sha256
from pathlib import Path
from typing import Protocol

import aiofiles
import git
import structlog
from tqdm import tqdm

from kodit.source.ignore import IgnorePatterns
from kodit.source.source_models import (
    Author,
    AuthorFileMapping,
    File,
    Source,
    SourceType,
)
from kodit.source.source_repository import SourceRepository


class WorkingCopyProvider(Protocol):
    """Protocol for providing working copies of sources."""

    async def prepare(self, uri: str) -> Path:
        """Prepare a working copy and return its path."""
        ...


class FileMetadataExtractor(Protocol):
    """Protocol for extracting file metadata."""

    async def extract(self, path: Path, source: Source) -> File:
        """Extract metadata from a file."""
        ...


class AuthorExtractor(Protocol):
    """Protocol for extracting author information."""

    async def extract(self, path: Path, source: Source) -> list[Author]:
        """Extract authors for a file."""
        ...


class SourceFactory(ABC):
    """Abstract base class for source factories."""

    def __init__(
        self,
        working_copy: WorkingCopyProvider,
        metadata_extractor: FileMetadataExtractor,
        author_extractor: AuthorExtractor,
        repository: SourceRepository,
    ) -> None:
        """Initialize the source factory."""
        self.working_copy = working_copy
        self.metadata_extractor = metadata_extractor
        self.author_extractor = author_extractor
        self.repository = repository
        self.log = structlog.get_logger(__name__)

    @abstractmethod
    async def create(self, uri: str) -> Source:
        """Create a source from a URI."""
        ...

    async def _process_files(self, source: Source, files: list[Path]) -> None:
        """Process files for a source."""
        for path in tqdm(files, total=len(files), leave=False):
            if not path.is_file():
                continue

            # Extract file metadata
            file_record = await self.metadata_extractor.extract(path, source)
            await self.repository.create_file(file_record)

            # Extract authors
            authors = await self.author_extractor.extract(path, source)
            for author in authors:
                await self.repository.upsert_author_file_mapping(
                    AuthorFileMapping(
                        author_id=author.id,
                        file_id=file_record.id,
                    )
                )


class GitSourceFactory(SourceFactory):
    """Factory for creating Git sources."""

    async def create(self, uri: str) -> Source:
        """Create a git source from a URI."""
        # Normalize the URI
        self.log.debug("Normalising git uri", uri=uri)
        with tempfile.TemporaryDirectory() as temp_dir:
            git.Repo.clone_from(uri, temp_dir)
            remote = git.Repo(temp_dir).remote()
            uri = remote.url

        # Check if source already exists
        self.log.debug("Checking if source already exists", uri=uri)
        source = await self.repository.get_source_by_uri(uri)

        if source:
            self.log.info("Source already exists, reusing...", source_id=source.id)
            return source

        # Prepare working copy
        clone_path = await self.working_copy.prepare(uri)

        # Create source record
        self.log.debug("Creating source", uri=uri, clone_path=str(clone_path))
        source = await self.repository.create_source(
            Source(
                uri=uri,
                cloned_path=str(clone_path),
                source_type=SourceType.GIT,
            )
        )

        # Get files to process using ignore patterns
        ignore_patterns = IgnorePatterns(clone_path)
        files = [
            f
            for f in clone_path.rglob("*")
            if f.is_file() and not ignore_patterns.should_ignore(f)
        ]

        # Process files
        self.log.info("Inspecting files", source_id=source.id, num_files=len(files))
        await self._process_files(source, files)

        return source


class FolderSourceFactory(SourceFactory):
    """Factory for creating folder sources."""

    async def create(self, uri: str) -> Source:
        """Create a folder source from a path."""
        directory = Path(uri).expanduser().resolve()

        # Check if source already exists
        source = await self.repository.get_source_by_uri(directory.as_uri())
        if source:
            self.log.info("Source already exists, reusing...", source_id=source.id)
            return source

        # Validate directory exists
        if not directory.exists():
            msg = f"Folder does not exist: {directory}"
            raise ValueError(msg)

        # Prepare working copy
        clone_path = await self.working_copy.prepare(directory.as_uri())

        # Create source record
        source = await self.repository.create_source(
            Source(
                uri=directory.as_uri(),
                cloned_path=str(clone_path),
                source_type=SourceType.FOLDER,
            )
        )

        # Get all files to process
        files = [f for f in clone_path.rglob("*") if f.is_file()]

        # Process files
        await self._process_files(source, files)

        return source


class GitWorkingCopyProvider:
    """Working copy provider for Git repositories."""

    def __init__(self, clone_dir: Path) -> None:
        """Initialize the provider."""
        self.clone_dir = clone_dir
        self.log = structlog.get_logger(__name__)

    async def prepare(self, uri: str) -> Path:
        """Prepare a Git working copy."""
        # Create a unique directory name for the clone
        clone_path = self.clone_dir / uri.replace("/", "_").replace(":", "_")
        clone_path.mkdir(parents=True, exist_ok=True)

        try:
            self.log.info("Cloning repository", uri=uri, clone_path=str(clone_path))
            git.Repo.clone_from(uri, clone_path)
        except git.GitCommandError as e:
            if "already exists and is not an empty directory" not in str(e):
                msg = f"Failed to clone repository: {e}"
                raise ValueError(msg) from e
            self.log.info("Repository already exists, reusing...", uri=uri)

        return clone_path


class FolderWorkingCopyProvider:
    """Working copy provider for local folders."""

    def __init__(self, clone_dir: Path) -> None:
        """Initialize the provider."""
        self.clone_dir = clone_dir

    async def prepare(self, uri: str) -> Path:
        """Prepare a folder working copy."""
        # Handle file:// URIs
        if uri.startswith("file://"):
            from urllib.parse import urlparse

            parsed = urlparse(uri)
            directory = Path(parsed.path).expanduser().resolve()
        else:
            directory = Path(uri).expanduser().resolve()

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

        return clone_path


class BaseFileMetadataExtractor:
    """Base class for file metadata extraction with common functionality."""

    async def extract(self, path: Path, source: Source) -> File:
        """Extract metadata from a file."""
        # Get timestamps - to be implemented by subclasses
        created_at, updated_at = await self._get_timestamps(path, source)

        # Read file content and calculate metadata
        async with aiofiles.open(path, "rb") as f:
            content = await f.read()
            mime_type = mimetypes.guess_type(path)
            sha = sha256(content).hexdigest()

            return File(
                created_at=created_at,
                updated_at=updated_at,
                source_id=source.id,
                cloned_path=str(path),
                mime_type=mime_type[0]
                if mime_type and mime_type[0]
                else "application/octet-stream",
                uri=path.as_uri(),
                sha256=sha,
                size_bytes=len(content),
            )

    async def _get_timestamps(
        self, path: Path, source: Source
    ) -> tuple[datetime, datetime]:
        """Get creation and modification timestamps. To be implemented by subclasses."""
        raise NotImplementedError


class GitFileMetadataExtractor(BaseFileMetadataExtractor):
    """Git-specific implementation for extracting file metadata."""

    async def _get_timestamps(
        self, path: Path, source: Source
    ) -> tuple[datetime, datetime]:
        """Get timestamps from Git history."""
        git_repo = git.Repo(source.cloned_path)
        commits = list(git_repo.iter_commits(paths=str(path), all=True))

        if commits:
            last_modified_at = commits[0].committed_datetime
            first_modified_at = commits[-1].committed_datetime
            return first_modified_at, last_modified_at
        # Fallback to current time if no commits found
        now = datetime.now(UTC)
        return now, now


class FolderFileMetadataExtractor(BaseFileMetadataExtractor):
    """Folder-specific implementation for extracting file metadata."""

    async def _get_timestamps(
        self,
        path: Path,
        source: Source,  # noqa: ARG002
    ) -> tuple[datetime, datetime]:
        """Get timestamps from file system."""
        stat = path.stat()
        file_created_at = datetime.fromtimestamp(stat.st_ctime, UTC)
        file_modified_at = datetime.fromtimestamp(stat.st_mtime, UTC)
        return file_created_at, file_modified_at


class GitAuthorExtractor:
    """Author extractor for Git repositories."""

    def __init__(self, repository: SourceRepository) -> None:
        """Initialize the extractor."""
        self.repository = repository

    async def extract(self, path: Path, source: Source) -> list[Author]:
        """Extract authors from a Git file."""
        authors: list[Author] = []
        git_repo = git.Repo(source.cloned_path)

        try:
            # Get the file's blame
            blames = git_repo.blame("HEAD", str(path))

            # Extract the blame's authors
            actors = [
                commit.author
                for blame in blames or []
                for commit in blame
                if isinstance(commit, git.Commit)
            ]

            # Get or create the authors in the database
            for actor in actors:
                if actor.email:
                    author = Author.from_actor(actor)
                    author = await self.repository.upsert_author(author)
                    authors.append(author)
        except git.GitCommandError:
            # Handle cases where file might not be tracked
            pass

        return authors


class NoOpAuthorExtractor:
    """No-op author extractor for sources that don't have author information."""

    async def extract(self, path: Path, source: Source) -> list[Author]:  # noqa: ARG002
        """Return empty list of authors."""
        return []
