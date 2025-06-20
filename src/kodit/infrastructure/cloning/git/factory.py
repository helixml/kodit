"""Factory for creating git-based working copies."""

import tempfile
from pathlib import Path

import git
import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import AuthorFileMapping, Source, SourceType
from kodit.domain.interfaces import NullProgressCallback, ProgressCallback
from kodit.domain.repositories import SourceRepository
from kodit.domain.services.ignore_service import IgnoreService
from kodit.domain.value_objects import ProgressEvent
from kodit.infrastructure.cloning.git.working_copy import GitWorkingCopyProvider
from kodit.infrastructure.cloning.metadata import (
    GitAuthorExtractor,
    GitFileMetadataExtractor,
)
from kodit.infrastructure.git.git_utils import sanitize_git_url
from kodit.infrastructure.ignore.ignore_pattern_provider import GitIgnorePatternProvider


class GitSourceFactory:
    """Factory for creating git-based working copies."""

    def __init__(
        self,
        repository: SourceRepository,
        working_copy: GitWorkingCopyProvider,
        session: AsyncSession,
    ) -> None:
        """Initialize the source factory."""
        self.log = structlog.get_logger(__name__)
        self.repository = repository
        self.working_copy = working_copy
        self.metadata_extractor = GitFileMetadataExtractor()
        self.author_extractor = GitAuthorExtractor(repository)
        self.session = session

    async def create(
        self, uri: str, progress_callback: ProgressCallback | None = None
    ) -> Source:
        """Create a git source from a URI."""
        # Use null callback if none provided
        if progress_callback is None:
            progress_callback = NullProgressCallback()

        # Normalize the URI
        # Never log the raw URI in production
        self.log.debug("Normalising git uri", uri="[REDACTED]" + uri[-4:])
        with tempfile.TemporaryDirectory() as temp_dir:
            git.Repo.clone_from(uri, temp_dir)
            remote = git.Repo(temp_dir).remote()
            uri = remote.url

        # Sanitize the URI to remove any credentials
        sanitized_uri = sanitize_git_url(uri)
        self.log.debug("Sanitized git uri", sanitized_uri=sanitized_uri)

        # Check if source already exists
        self.log.debug("Checking if source already exists", uri=sanitized_uri)
        source = await self.repository.get_by_uri(sanitized_uri)

        if source:
            self.log.info("Source already exists, reusing...", source_id=source.id)
            return source

        # Prepare working copy (use original URI for cloning, sanitized for storage)
        clone_path = await self.working_copy.prepare(uri)

        # Create source record
        self.log.debug("Creating source", uri=sanitized_uri, clone_path=str(clone_path))
        source = await self.repository.save(
            Source(
                uri=sanitized_uri,
                cloned_path=str(clone_path),
                source_type=SourceType.GIT,
            )
        )

        # Commit source creation so we get an ID for foreign key relationships
        await self.session.commit()

        # Get files to process using ignore patterns
        ignore_provider = GitIgnorePatternProvider(clone_path)
        ignore_service = IgnoreService(ignore_provider)
        files = [
            f
            for f in clone_path.rglob("*")
            if f.is_file() and not ignore_service.should_ignore(f)
        ]

        # Process files
        self.log.info("Inspecting files", source_id=source.id, num_files=len(files))
        await self._process_files(source, files, progress_callback)

        # Commit file processing
        await self.session.commit()

        return source

    async def _process_files(
        self, source: Source, files: list[Path], progress_callback: ProgressCallback
    ) -> None:
        """Process files for a source."""
        total_files = len(files)

        # Notify start of operation
        await progress_callback.on_progress(
            ProgressEvent(
                operation="process_files",
                current=0,
                total=total_files,
                message="Processing files...",
            )
        )

        for i, path in enumerate(files, 1):
            if not path.is_file():
                continue

            # Extract file metadata
            file_record = await self.metadata_extractor.extract(path, source)
            await self.repository.create_file(file_record)

            # Extract authors
            authors = await self.author_extractor.extract(path, source)

            # Commit authors so they get IDs before creating mappings
            if authors:
                await self.session.commit()

            for author in authors:
                await self.repository.upsert_author_file_mapping(
                    AuthorFileMapping(
                        author_id=author.id,
                        file_id=file_record.id,
                    )
                )

            # Update progress
            await progress_callback.on_progress(
                ProgressEvent(
                    operation="process_files",
                    current=i,
                    total=total_files,
                    message=f"Processing {path.name}...",
                )
            )

        # Notify completion
        await progress_callback.on_complete("process_files")
