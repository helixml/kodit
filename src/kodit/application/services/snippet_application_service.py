"""Application service for snippet operations."""

from pathlib import Path
from typing import Any

import structlog
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.commands.snippet_commands import (
    CreateIndexSnippetsCommand,
    ExtractSnippetsCommand,
    ListSnippetsCommand,
)
from kodit.domain.entities import Snippet
from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.repositories import FileRepository, SnippetRepository
from kodit.domain.services.snippet_extraction_service import (
    SnippetExtractionDomainService,
)
from kodit.domain.value_objects import SnippetExtractionRequest, SnippetListItem
from kodit.reporting import Reporter


class SnippetApplicationService:
    """Application service for snippet operations."""

    def __init__(
        self,
        snippet_extraction_service: SnippetExtractionDomainService,
        snippet_repository: SnippetRepository,
        file_repository: FileRepository,
        session: AsyncSession,
    ) -> None:
        """Initialize the snippet application service.

        Args:
            snippet_extraction_service: Domain service for snippet extraction
            snippet_repository: Repository for snippet persistence
            file_repository: Repository for file operations
            session: The database session for transaction management

        """
        self.snippet_extraction_service = snippet_extraction_service
        self.snippet_repository = snippet_repository
        self.file_repository = file_repository
        self.session = session
        self.log = structlog.get_logger(__name__)

    async def extract_snippets_from_file(
        self, command: ExtractSnippetsCommand
    ) -> list[Snippet]:
        """Application use case: extract snippets from a single file.

        Args:
            command: The extract snippets command

        Returns:
            List of extracted snippets

        """
        request = SnippetExtractionRequest(command.file_path, command.strategy)
        result = await self.snippet_extraction_service.extract_snippets(request)

        # Convert domain result to persistence model
        return [
            Snippet(
                file_id=0, index_id=0, content=snippet_text
            )  # IDs will be set later
            for snippet_text in result.snippets
        ]

    def _should_process_file(self, file: Any) -> bool:
        """Check if a file should be processed for snippet extraction.

        Args:
            file: The file to check

        Returns:
            True if the file should be processed

        """
        # Skip unsupported file types
        mime_blacklist = ["unknown/unknown"]
        return file.mime_type not in mime_blacklist

    async def _extract_snippets_from_file(
        self, file: Any, strategy: SnippetExtractionStrategy
    ) -> list[str]:
        """Extract snippets from a single file."""
        command = ExtractSnippetsCommand(
            file_path=Path(file.cloned_path),
            strategy=strategy,
        )
        snippets = await self.extract_snippets_from_file(command)
        return [snippet.content for snippet in snippets]

    async def create_snippets_for_index(
        self,
        command: CreateIndexSnippetsCommand,
        progress_callback: ProgressCallback | None = None,
    ) -> None:
        """Create snippets for all files in an index.

        Args:
            command: The create index snippets command
            progress_callback: Optional progress callback for reporting progress

        """
        files = await self.file_repository.get_files_for_index(command.index_id)

        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "create_snippets", len(files), "Creating snippets from files..."
        )

        for i, file in enumerate(files, 1):
            try:
                if not self._should_process_file(file):
                    continue

                snippet_contents = await self._extract_snippets_from_file(
                    file, command.strategy
                )
                for snippet_content in snippet_contents:
                    snippet = Snippet(
                        file_id=file.id,
                        index_id=command.index_id,
                        content=snippet_content,
                    )
                    await self.snippet_repository.save(snippet)

            except (OSError, ValueError) as e:
                self.log.debug(
                    "Skipping file",
                    file=file.cloned_path,
                    error=str(e),
                )
                continue

            await reporter.step(
                "create_snippets",
                current=i,
                total=len(files),
                message=f"Processing {file.cloned_path}...",
            )

        # Commit all snippet creations in a single transaction
        await self.session.commit()
        await reporter.done("create_snippets")

    async def list_snippets(
        self, command: ListSnippetsCommand
    ) -> list[SnippetListItem]:
        """List snippets with optional filtering.

        Args:
            command: The list snippets command with optional file path and source URI
            filters

        Returns:
            List of SnippetListItem instances matching the criteria

        """
        return list(
            await self.snippet_repository.list_snippets(
                command.file_path, command.source_uri
            )
        )
