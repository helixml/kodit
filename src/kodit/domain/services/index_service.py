"""Pure domain service for Index aggregate operations."""

from pathlib import Path
from typing import TYPE_CHECKING

import structlog
from pydantic import AnyUrl

from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.models import entities as domain_entities
from kodit.domain.models.protocols import IndexRepository
from kodit.domain.models.value_objects import (
    SnippetContent,
    SnippetContentType,
    SourceType,
)
from kodit.domain.value_objects import SnippetExtractionRequest
from kodit.reporting import Reporter

if TYPE_CHECKING:
    from kodit.domain.services.snippet_extraction_service import (
        SnippetExtractionDomainService,
    )


class IndexDomainService:
    """Pure domain service for Index aggregate operations.

    This service handles the full lifecycle of code indexing:
    - Creating indexes for source repositories
    - Cloning and processing source files
    - Extracting and enriching code snippets
    - Managing the complete Index aggregate
    """

    def __init__(
        self,
        index_repository: IndexRepository,
        snippet_extraction_service: "SnippetExtractionDomainService",
    ) -> None:
        """Initialize the index domain service.

        Args:
            index_repository: Repository for Index aggregate persistence
            snippet_extraction_service: Service for extracting snippets from files

        """
        self._index_repository = index_repository
        self._snippet_extraction_service = snippet_extraction_service
        self.log = structlog.get_logger(__name__)

    async def create_index(self, uri: AnyUrl) -> domain_entities.Index:
        """Create a new index for a source repository.

        Args:
            uri: The URI of the source repository to index

        Returns:
            The created Index aggregate with minimal structure

        """
        self.log.info("Creating index", uri=str(uri))

        # Check if index already exists
        existing_index = await self._index_repository.get_by_uri(uri)
        if existing_index:
            self.log.info(
                "Index already exists", uri=str(uri), index_id=existing_index.id
            )
            return existing_index

        # Create new index
        index = await self._index_repository.create(uri)
        self.log.info("Index created", uri=str(uri), index_id=index.id)

        return index

    async def clone_and_populate_working_copy(
        self,
        index: domain_entities.Index,
        local_path: Path,
        source_type: SourceType,
        progress_callback: ProgressCallback | None = None,
    ) -> domain_entities.Index:
        """Clone the source repository and populate the working copy with files.

        Args:
            index: The Index aggregate to populate
            local_path: Local path where the source should be cloned
            source_type: Type of the source (GIT, FOLDER, etc.)
            progress_callback: Optional callback for progress reporting

        Returns:
            Updated Index aggregate with populated working copy

        """
        from datetime import UTC, datetime

        self.log.info(
            "Cloning and populating working copy",
            index_id=index.id,
            uri=str(index.source.working_copy.remote_uri),
            local_path=str(local_path)
        )

        reporter = Reporter(self.log, progress_callback)
        await reporter.start("clone_source", 100, "Cloning source repository...")

        # TODO: Implement actual cloning logic here
        # For now, we'll assume the source is already available at local_path

        # Scan for files in the cloned directory
        files = []
        if local_path.exists():
            file_paths = list(local_path.rglob("*"))
            file_count = len([p for p in file_paths if p.is_file()])

            await reporter.start("scan_files", file_count, "Scanning files...")

            for i, file_path in enumerate(file_paths):
                if not file_path.is_file():
                    continue

                # Create domain file entity
                try:
                    relative_path = file_path.relative_to(local_path)
                    now = datetime.now(UTC)

                    # Calculate file hash
                    import hashlib
                    sha256_hash = hashlib.sha256()
                    with file_path.open("rb") as f:
                        for chunk in iter(lambda: f.read(4096), b""):
                            sha256_hash.update(chunk)

                    domain_file = domain_entities.File(
                        id=0,  # Will be assigned by repository
                        created_at=now,
                        updated_at=now,
                        uri=AnyUrl(f"file://{relative_path}"),
                        sha256=sha256_hash.hexdigest(),
                        authors=[]  # Will be populated later via git blame or similar
                    )
                    files.append(domain_file)

                except (OSError, ValueError) as e:
                    self.log.debug("Skipping file", file=str(file_path), error=str(e))
                    continue

                await reporter.step(
                    "scan_files", i + 1, file_count, f"Scanned {file_path.name}"
                )

        await reporter.done("scan_files")

        # Create updated working copy
        now = datetime.now(UTC)
        updated_working_copy = domain_entities.WorkingCopy(
            created_at=index.source.working_copy.created_at,
            updated_at=now,
            remote_uri=index.source.working_copy.remote_uri,
            cloned_path=local_path,
            source_type=source_type,
            files=files
        )

        # Set the working copy in the repository
        await self._index_repository.set_working_copy(index.id, updated_working_copy)

        await reporter.done("clone_source")

        # Return updated index
        return await self._index_repository.get(index.id) or index

    async def extract_snippets(
        self,
        index: domain_entities.Index,
        strategy: SnippetExtractionStrategy = SnippetExtractionStrategy.METHOD_BASED,
        progress_callback: ProgressCallback | None = None,
    ) -> domain_entities.Index:
        """Extract code snippets from files in the index.

        Args:
            index: The Index aggregate to extract snippets from
            strategy: The extraction strategy to use
            progress_callback: Optional callback for progress reporting

        Returns:
            Updated Index aggregate with extracted snippets

        """
        self.log.info(
            "Extracting snippets",
            index_id=index.id,
            file_count=len(index.source.working_copy.files),
            strategy=strategy.value
        )

        files = index.source.working_copy.files
        snippets = []

        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "extract_snippets", len(files), "Extracting code snippets..."
        )

        for i, domain_file in enumerate(files, 1):
            try:
                # Determine file path for extraction
                file_path = (
                    index.source.working_copy.cloned_path
                    / Path(domain_file.uri.path or "")
                )

                if not self._should_process_file(file_path):
                    continue

                # Extract snippets from file
                request = SnippetExtractionRequest(file_path, strategy)
                result = await self._snippet_extraction_service.extract_snippets(
                    request
                )

                # Create snippet entities
                from datetime import UTC, datetime
                now = datetime.now(UTC)

                for snippet_text in result.snippets:
                    # Create snippet contents
                    contents = [
                        SnippetContent(
                            type=SnippetContentType.ORIGINAL,
                            value=snippet_text,
                            language=result.language
                        )
                    ]

                    snippet = domain_entities.Snippet(
                        id=0,  # Will be assigned by repository
                        created_at=now,
                        updated_at=now,
                        contents=contents
                    )
                    snippets.append(snippet)

            except (OSError, ValueError) as e:
                self.log.debug(
                    "Skipping file for snippet extraction",
                    file_uri=str(domain_file.uri),
                    error=str(e)
                )
                continue

            await reporter.step(
                "extract_snippets",
                i,
                len(files),
                f"Processed {domain_file.uri.path}"
            )

        # Add snippets to the index
        if snippets:
            await self._index_repository.add_snippets(index.id, snippets)

        await reporter.done("extract_snippets")

        # Return updated index
        return await self._index_repository.get(index.id) or index

    async def enrich_snippets_with_summaries(
        self,
        index: domain_entities.Index,
        progress_callback: ProgressCallback | None = None,
    ) -> domain_entities.Index:
        """Enrich snippets with AI-generated summaries.

        Args:
            index: The Index aggregate containing snippets to enrich
            progress_callback: Optional callback for progress reporting

        Returns:
            Updated Index aggregate with enriched snippets

        """
        self.log.info("Enriching snippets with summaries", index_id=index.id)

        # TODO: Implement summary generation
        # This would involve:
        # 1. Loading snippets from the index
        # 2. Generating summaries using an AI service
        # 3. Creating new SnippetContent with type SUMMARY
        # 4. Updating the snippets in the repository

        reporter = Reporter(self.log, progress_callback)
        await reporter.start("enrich_snippets", 1, "Enriching snippets...")

        # Placeholder implementation
        await reporter.done("enrich_snippets")

        return index

    async def get_index_by_uri(self, uri: AnyUrl) -> domain_entities.Index | None:
        """Get an index by source URI.

        Args:
            uri: The URI of the source repository

        Returns:
            The Index aggregate if found, None otherwise

        """
        return await self._index_repository.get_by_uri(uri)

    async def get_index_by_id(self, index_id: int) -> domain_entities.Index | None:
        """Get an index by ID.

        Args:
            index_id: The ID of the index

        Returns:
            The Index aggregate if found, None otherwise

        """
        return await self._index_repository.get(index_id)

    def _should_process_file(self, file_path: Path) -> bool:
        """Check if a file should be processed for snippet extraction.

        Args:
            file_path: The path to the file

        Returns:
            True if the file should be processed

        """
        # Skip hidden files and directories
        if any(part.startswith(".") for part in file_path.parts):
            return False

        # Skip binary files (basic check)
        try:
            with file_path.open(encoding="utf-8") as f:
                f.read(1024)  # Try to read first 1KB as text
            return True
        except (UnicodeDecodeError, OSError):
            return False

