"""Application service for snippet operations."""

from pathlib import Path

import structlog
from tqdm import tqdm

from kodit.application.commands.snippet_commands import (
    CreateIndexSnippetsCommand,
    ExtractSnippetsCommand,
)
from kodit.domain.models import Snippet, SnippetExtractionRequest
from kodit.domain.repositories import FileRepository, SnippetRepository
from kodit.domain.services.snippet_extraction_service import (
    SnippetExtractionDomainService,
)


class SnippetApplicationService:
    """Application service for snippet operations."""

    def __init__(
        self,
        snippet_extraction_service: SnippetExtractionDomainService,
        snippet_repository: SnippetRepository,
        file_repository: FileRepository,
    ) -> None:
        """Initialize the snippet application service.

        Args:
            snippet_extraction_service: Domain service for snippet extraction
            snippet_repository: Repository for snippet persistence
            file_repository: Repository for file operations

        """
        self.snippet_extraction_service = snippet_extraction_service
        self.snippet_repository = snippet_repository
        self.file_repository = file_repository
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
        snippets = [
            Snippet(
                file_id=0, index_id=0, content=snippet_text
            )  # IDs will be set later
            for snippet_text in result.snippets
        ]

        return snippets

    async def create_snippets_for_index(
        self, command: CreateIndexSnippetsCommand
    ) -> None:
        """Application use case: create snippets for all files in an index.

        Args:
            command: The create index snippets command

        """
        files = await self.file_repository.get_files_for_index(command.index_id)

        if not files:
            self.log.warning(
                "No files to create snippets for", index_id=command.index_id
            )
            return

        for file in tqdm(files, total=len(files), leave=False):
            if self._should_process_file(file):
                try:
                    extract_command = ExtractSnippetsCommand(
                        file_path=Path(file.cloned_path),
                        strategy=command.strategy,
                    )
                    snippets = await self.extract_snippets_from_file(extract_command)

                    # Save snippets with file and index associations
                    for snippet in snippets:
                        snippet.file_id = file.id
                        snippet.index_id = command.index_id
                        await self.snippet_repository.save(snippet)

                except Exception as e:
                    self.log.debug(
                        "Skipping file",
                        file=file.cloned_path,
                        error=str(e),
                    )
                    continue

    def _should_process_file(self, file) -> bool:
        """Check if a file should be processed for snippet extraction.

        Args:
            file: The file to check

        Returns:
            True if the file should be processed

        """
        # Skip unsupported file types
        mime_blacklist = ["unknown/unknown"]
        return file.mime_type not in mime_blacklist
