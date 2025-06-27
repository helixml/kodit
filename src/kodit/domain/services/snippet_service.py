"""Domain service for snippet operations."""

from pathlib import Path
from typing import Any

import structlog

from kodit.domain.entities import Snippet
from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.repositories import FileRepository, SnippetRepository
from kodit.domain.services.snippet_extraction_service import (
    SnippetExtractionDomainService,
)
from kodit.domain.value_objects import (
    MultiSearchRequest,
    MultiSearchResult,
    SnippetExtractionRequest,
    SnippetListItem,
)
from kodit.reporting import Reporter


class SnippetDomainService:
    """Domain service for snippet-related operations.

    This service consolidates snippet operations that were previously
    spread between application services. It handles:
    - Snippet extraction from files
    - Snippet persistence
    - Snippet querying and filtering
    """

    def __init__(
        self,
        snippet_extraction_service: SnippetExtractionDomainService,
        snippet_repository: SnippetRepository,
        file_repository: FileRepository,
    ) -> None:
        """Initialize the snippet domain service.

        Args:
            snippet_extraction_service: Service for extracting snippets from files
            snippet_repository: Repository for snippet persistence
            file_repository: Repository for file operations

        """
        self.snippet_extraction_service = snippet_extraction_service
        self.snippet_repository = snippet_repository
        self.file_repository = file_repository
        self.log = structlog.get_logger(__name__)

    async def extract_and_create_snippets(
        self,
        index_id: int,
        strategy: SnippetExtractionStrategy,
        progress_callback: ProgressCallback | None = None,
    ) -> list[Snippet]:
        """Extract snippets from all files in an index and persist them.

        This method combines the extraction and persistence logic that was
        previously split between domain and application services.

        Args:
            index_id: The ID of the index to create snippets for
            strategy: The extraction strategy to use
            progress_callback: Optional callback for progress reporting

        Returns:
            List of created Snippet entities with IDs assigned

        """
        files = await self.file_repository.get_files_for_index(index_id)
        created_snippets = []

        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "create_snippets", len(files), "Creating snippets from files..."
        )

        for i, file in enumerate(files, 1):
            if not self._should_process_file(file):
                continue

            try:
                # Extract snippets from file
                request = SnippetExtractionRequest(Path(file.cloned_path), strategy)
                result = await self.snippet_extraction_service.extract_snippets(request)

                # Create and persist snippet entities
                for snippet_content in result.snippets:
                    snippet = Snippet(
                        file_id=file.id,
                        index_id=index_id,
                        content=snippet_content,
                    )
                    saved_snippet = await self.snippet_repository.save(snippet)
                    created_snippets.append(saved_snippet)

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

        await reporter.done("create_snippets")
        return created_snippets

    async def get_snippets_for_index(self, index_id: int) -> list[Snippet]:
        """Get all snippets for a specific index.

        Args:
            index_id: The ID of the index

        Returns:
            List of Snippet entities for the index

        """
        # This delegates to the repository but provides a domain-level interface
        return list(await self.snippet_repository.get_by_index(index_id))

    async def update_snippet_content(self, snippet_id: int, content: str) -> None:
        """Update the content of an existing snippet.

        Args:
            snippet_id: The ID of the snippet to update
            content: The new content for the snippet

        """
        # Get the snippet first to ensure it exists
        snippet = await self.snippet_repository.get(snippet_id)
        if not snippet:
            msg = f"Snippet not found: {snippet_id}"
            raise ValueError(msg)

        # Update the content
        snippet.content = content
        await self.snippet_repository.save(snippet)

    async def delete_snippets_for_index(self, index_id: int) -> None:
        """Delete all snippets for a specific index.

        Args:
            index_id: The ID of the index

        """
        await self.snippet_repository.delete_by_index(index_id)

    async def search_snippets(
        self, request: MultiSearchRequest
    ) -> list[SnippetListItem]:
        """Search snippets with filters.

        Args:
            request: The search request containing filters

        Returns:
            List of matching snippet items

        """
        return list(await self.snippet_repository.search(request))

    async def list_snippets(
        self, file_path: str | None = None, source_uri: str | None = None
    ) -> list[MultiSearchResult]:
        """List snippets with optional filtering.

        Args:
            file_path: Optional file path to filter by
            source_uri: Optional source URI to filter by

        Returns:
            List of search results matching the criteria

        """
        snippet_items = await self.snippet_repository.list_snippets(
            file_path, source_uri
        )
        # Convert SnippetListItem to MultiSearchResult for unified display format
        return [
            MultiSearchResult(
                id=item.id,
                uri=item.source_uri,
                content=item.content,
                original_scores=[],
            )
            for item in snippet_items
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
