"""Metadata extraction for cloned sources."""

import mimetypes
from datetime import UTC, datetime
from hashlib import sha256
from pathlib import Path
from typing import Any

import aiofiles
import git
import structlog

from kodit.domain.entities import Author, File, Source


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
                extension=path.suffix.removeprefix(".").lower(),
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

    def __init__(self, repository: Any) -> None:
        """Initialize the extractor."""
        self.repository = repository
        self.log = structlog.get_logger(__name__)

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
