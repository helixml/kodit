"""Factory for creating git-based working copies."""

import tempfile
from pathlib import Path

import git
import structlog

from kodit.domain.models import AuthorFileMapping, Source, SourceType
from kodit.domain.repositories import SourceRepository
from kodit.domain.services.ignore_service import IgnoreService
from kodit.infrastructure.cloning.git.working_copy import GitWorkingCopyProvider
from kodit.infrastructure.cloning.metadata import (
    GitAuthorExtractor,
    GitFileMetadataExtractor,
)
from kodit.infrastructure.ignore import GitIgnorePatternProvider


class GitSourceFactory:
    """Factory for creating git-based working copies."""

    def __init__(
        self,
        repository: SourceRepository,
        working_copy: GitWorkingCopyProvider,
    ) -> None:
        """Initialize the source factory."""
        self.log = structlog.get_logger(__name__)
        self.repository = repository
        self.working_copy = working_copy
        self.metadata_extractor = GitFileMetadataExtractor()
        self.author_extractor = GitAuthorExtractor(repository)

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
        source = await self.repository.get_by_uri(uri)

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
        ignore_provider = GitIgnorePatternProvider(clone_path)
        ignore_service = IgnoreService(ignore_provider)
        files = [
            f
            for f in clone_path.rglob("*")
            if f.is_file() and not ignore_service.should_ignore(f)
        ]

        # Process files
        self.log.info("Inspecting files", source_id=source.id, num_files=len(files))
        await self._process_files(source, files)

        return source

    async def _process_files(self, source: Source, files: list[Path]) -> None:
        """Process files for a source."""
        for path in files:
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
