"""Metadata extraction for cloned sources."""

import mimetypes
from datetime import UTC, datetime
from hashlib import sha256
from pathlib import Path

import aiofiles
import git
from pydantic import AnyUrl

from kodit.domain.entities import Author, File
from kodit.domain.value_objects import FileProcessingStatus, SourceType


class FileMetadataExtractor:
    """File metadata extractor."""

    def __init__(self, source_type: SourceType) -> None:
        """Initialize the extractor."""
        self.source_type = source_type

    async def extract(self, file_path: Path) -> File:
        """Extract metadata from a file."""
        if self.source_type == SourceType.GIT:
            created_at, updated_at = await self._get_git_timestamps(file_path)
        else:
            created_at, updated_at = await self._get_file_system_timestamps(file_path)

        # Read file content and calculate metadata
        async with aiofiles.open(file_path, "rb") as f:
            content = await f.read()
            mime_type = mimetypes.guess_type(file_path)
            sha = sha256(content).hexdigest()
            if self.source_type == SourceType.GIT:
                authors = await self._extract_git_authors(file_path)
            else:
                authors = []

            return File(
                created_at=created_at,
                updated_at=updated_at,
                uri=AnyUrl(file_path.resolve().absolute().as_uri()),
                mime_type=mime_type[0]
                if mime_type and mime_type[0]
                else "application/octet-stream",
                sha256=sha,
                authors=authors,
                file_processing_status=FileProcessingStatus.ADDED,
            )

    async def _get_git_timestamps(self, file_path: Path) -> tuple[datetime, datetime]:
        """Get timestamps from Git history."""
        git_repo = git.Repo(file_path.parent, search_parent_directories=True)
        commits = list(git_repo.iter_commits(paths=str(file_path), all=True))

        if commits:
            last_modified_at = commits[0].committed_datetime
            first_modified_at = commits[-1].committed_datetime
            return first_modified_at, last_modified_at
        # Fallback to current time if no commits found
        now = datetime.now(UTC)
        return now, now

    async def _get_file_system_timestamps(
        self,
        file_path: Path,
    ) -> tuple[datetime, datetime]:
        """Get timestamps from file system."""
        stat = file_path.stat()
        file_created_at = datetime.fromtimestamp(stat.st_ctime, UTC)
        file_modified_at = datetime.fromtimestamp(stat.st_mtime, UTC)
        return file_created_at, file_modified_at

    async def _extract_git_authors(self, file_path: Path) -> list[Author]:
        """Extract authors from a Git file."""
        git_repo = git.Repo(file_path.parent, search_parent_directories=True)

        try:
            # Get the file's blame
            blames = git_repo.blame("HEAD", str(file_path))

            # Extract the blame's authors
            actors = [
                commit.author
                for blame in blames or []
                for commit in blame
                if isinstance(commit, git.Commit)
            ]

            # Get or create the authors in the database
            return [
                Author(name=actor.name or "", email=actor.email or "")
                for actor in actors
            ]
        except git.GitCommandError:
            # Handle cases where file might not be tracked
            return []
