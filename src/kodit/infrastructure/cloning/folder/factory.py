"""Factory for creating folder-based working copies."""

from pathlib import Path

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities import AuthorFileMapping, Source, SourceType
from kodit.domain.interfaces import NullProgressCallback, ProgressCallback
from kodit.domain.repositories import SourceRepository
from kodit.domain.value_objects import ProgressEvent
from kodit.infrastructure.cloning.folder.working_copy import FolderWorkingCopyProvider
from kodit.infrastructure.cloning.metadata import (
    FolderFileMetadataExtractor,
    NoOpAuthorExtractor,
)


class FolderSourceFactory:
    """Factory for creating folder sources."""

    def __init__(
        self,
        repository: SourceRepository,
        working_copy: FolderWorkingCopyProvider,
        session: AsyncSession,
    ) -> None:
        """Initialize the source factory."""
        self.log = structlog.get_logger(__name__)
        self.repository = repository
        self.working_copy = working_copy
        self.metadata_extractor = FolderFileMetadataExtractor()
        self.author_extractor = NoOpAuthorExtractor()
        self.session = session

    async def create(
        self, uri: str, progress_callback: ProgressCallback | None = None
    ) -> Source:
        """Create a folder source from a path."""
        # Use null callback if none provided
        if progress_callback is None:
            progress_callback = NullProgressCallback()

        directory = Path(uri).expanduser().resolve()

        # Check if source already exists
        source = await self.repository.get_by_uri(directory.as_uri())
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
        source = await self.repository.save(
            Source(
                uri=directory.as_uri(),
                cloned_path=str(clone_path),
                source_type=SourceType.FOLDER,
            )
        )

        # Commit source creation so we get an ID for foreign key relationships
        await self.session.commit()

        # Get all files to process
        files = [f for f in clone_path.rglob("*") if f.is_file()]

        # Process files
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
