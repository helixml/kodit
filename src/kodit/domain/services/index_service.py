"""Pure domain service for Index aggregate operations."""

from abc import ABC, abstractmethod
from collections.abc import Mapping
from pathlib import Path

import structlog
from pydantic import AnyUrl

import kodit.domain.entities as domain_entities
from kodit.domain.interfaces import ProgressCallback
from kodit.domain.protocols import IndexRepository
from kodit.domain.services.enrichment_service import EnrichmentDomainService
from kodit.domain.value_objects import (
    EnrichmentIndexRequest,
    EnrichmentRequest,
    SnippetExtractionRequest,
    SnippetExtractionResult,
    SnippetExtractionStrategy,
)
from kodit.infrastructure.cloning.git.working_copy import GitWorkingCopyProvider
from kodit.infrastructure.cloning.metadata import FileMetadataExtractor
from kodit.infrastructure.git.git_utils import is_valid_clone_target
from kodit.infrastructure.ignore.ignore_pattern_provider import GitIgnorePatternProvider
from kodit.reporting import Reporter
from kodit.utils.path_utils import path_from_uri


class LanguageDetectionService(ABC):
    """Abstract interface for language detection service."""

    @abstractmethod
    async def detect_language(self, file_path: Path) -> str:
        """Detect the programming language of a file."""


class SnippetExtractor(ABC):
    """Abstract interface for snippet extraction."""

    @abstractmethod
    async def extract(self, file_path: Path, language: str) -> list[str]:
        """Extract snippets from a file."""


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
        language_detector: LanguageDetectionService,
        snippet_extractors: Mapping[SnippetExtractionStrategy, SnippetExtractor],
        enrichment_service: EnrichmentDomainService,
        clone_dir: Path,
    ) -> None:
        """Initialize the index domain service.

        Args:
            index_repository: Repository for Index aggregate persistence
            snippet_extraction_service: Service for extracting snippets from files

        """
        self._index_repository = index_repository
        self._clone_dir = clone_dir
        self._language_detector = language_detector
        self._snippet_extractors = snippet_extractors
        self._enrichment_service = enrichment_service
        self.log = structlog.get_logger(__name__)

    async def create_index(
        self,
        uri_or_path_like: str,  # Must include user/pass, etc
        progress_callback: ProgressCallback | None = None,
    ) -> domain_entities.Index:
        """Create a new index and populate the working copy with files."""
        sanitized_uri = self._sanitize_uri(uri_or_path_like)
        self.log.debug("Creating index", uri=str(sanitized_uri))

        # Check if index already exists
        existing_index = await self._index_repository.get_by_uri(sanitized_uri)
        if existing_index:
            self.log.debug(
                "Index already exists",
                uri=str(sanitized_uri),
                index_id=existing_index.id,
            )
            return existing_index

        # Clone the source repository
        if is_valid_clone_target(uri_or_path_like):
            source_type = domain_entities.SourceType.GIT
            sanitized_uri = domain_entities.WorkingCopy.sanitize_git_url(
                uri_or_path_like
            )
            git_working_copy_provider = GitWorkingCopyProvider(self._clone_dir)
            local_path = await git_working_copy_provider.prepare(uri_or_path_like)
        elif Path(uri_or_path_like).is_dir():
            source_type = domain_entities.SourceType.FOLDER
            sanitized_uri = domain_entities.WorkingCopy.sanitize_local_path(
                uri_or_path_like
            )
            local_path = path_from_uri(str(sanitized_uri))
        else:
            raise ValueError(f"Unsupported source: {uri_or_path_like}")

        # Get files to process using ignore patterns
        ignore_provider = GitIgnorePatternProvider(local_path)
        files: list[domain_entities.File] = []
        file_paths = [
            f
            for f in local_path.rglob("*")
            if f.is_file() and not ignore_provider.should_ignore(f)
        ]
        file_count = len(file_paths)
        if file_count == 0:
            self.log.info("No files to index", uri=str(sanitized_uri))
            raise ValueError("No files to index")

        reporter = Reporter(self.log, progress_callback)
        await reporter.start("scan_files", file_count, "Scanning files...")

        metadata_extractor = FileMetadataExtractor(source_type)

        for i, file_path in enumerate(file_paths):
            # Create domain file entity
            try:
                files.append(await metadata_extractor.extract(file_path=file_path))
            except (OSError, ValueError) as e:
                self.log.debug("Skipping file", file=str(file_path), error=str(e))
                continue

            await reporter.step(
                "scan_files", i + 1, file_count, f"Scanned {file_path.name}"
            )

        await reporter.done("scan_files")

        # Create updated working copy
        working_copy = domain_entities.WorkingCopy(
            remote_uri=sanitized_uri,
            cloned_path=local_path,
            source_type=source_type,
            files=files,
        )

        return await self._index_repository.create(sanitized_uri, working_copy)

    async def update_index_timestamp(self, index_id: int) -> None:
        """Update the timestamp of an index.

        Args:
            index_id: The ID of the index to update.

        """
        await self._index_repository.update_index_timestamp(index_id)

    async def delete_snippets(self, index_id: int) -> None:
        """Delete all snippets from an index."""
        await self._index_repository.delete_snippets(index_id)

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
        file_count = len(index.source.working_copy.files)

        self.log.info(
            "Extracting snippets",
            index_id=index.id,
            file_count=file_count,
            strategy=strategy.value,
        )

        files = index.source.working_copy.files
        snippets = []

        reporter = Reporter(self.log, progress_callback)
        await reporter.start(
            "extract_snippets", len(files), "Extracting code snippets..."
        )

        for i, domain_file in enumerate(files, 1):
            try:
                # Extract snippets from file
                request = SnippetExtractionRequest(
                    file_path=domain_file.as_path(), strategy=strategy
                )
                result = await self._extract_snippets(request)
                for snippet_text in result.snippets:
                    snippet = domain_entities.Snippet(
                        derives_from=[domain_file],
                    )
                    snippet.add_original_content(snippet_text, result.language)
                    snippets.append(snippet)

            except (OSError, ValueError) as e:
                self.log.debug(
                    "Skipping file for snippet extraction",
                    file_uri=str(domain_file.uri),
                    error=str(e),
                )
                continue

            await reporter.step(
                "extract_snippets", i, len(files), f"Processed {domain_file.uri.path}"
            )

        # Add snippets to the index
        if snippets:
            await self._index_repository.add_snippets(index.id, snippets)

        await reporter.done("extract_snippets")

        # Return updated index
        return await self._index_repository.get(index.id) or index

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

    async def enrich_snippets(
        self,
        index: domain_entities.Index,
        progress_callback: ProgressCallback | None = None,
    ) -> domain_entities.Index:
        """Enrich snippets with AI-generated summaries."""
        if not index.id:
            raise ValueError("Index has no ID")

        if not index.snippets or len(index.snippets) == 0:
            return index

        reporter = Reporter(self.log, progress_callback)
        await reporter.start("enrichment", len(index.snippets), "Enriching snippets...")

        snippet_map = {snippet.id: snippet for snippet in index.snippets if snippet.id}

        enrichment_request = EnrichmentIndexRequest(
            requests=[
                EnrichmentRequest(snippet_id=snippet_id, text=snippet.original_text())
                for snippet_id, snippet in snippet_map.items()
            ]
        )

        processed = 0
        async for result in self._enrichment_service.enrich_documents(
            enrichment_request
        ):
            snippet_map[result.snippet_id].add_summary(result.text)

            processed += 1
            await reporter.step(
                "enrichment", processed, len(index.snippets), "Enriching snippets..."
            )

        await self._index_repository.update_snippets(
            index.id, list(snippet_map.values())
        )
        await reporter.done("enrichment")
        new_index = await self._index_repository.get(index.id)
        if not new_index:
            raise ValueError("Index not found after enrichment")
        return new_index

    async def _extract_snippets(
        self, request: SnippetExtractionRequest
    ) -> SnippetExtractionResult:
        # Domain logic: validate file exists
        if not request.file_path.exists():
            raise ValueError(f"File does not exist: {request.file_path}")

        # Domain logic: detect language
        language = await self._language_detector.detect_language(request.file_path)

        # Domain logic: choose strategy and extractor
        if request.strategy not in self._snippet_extractors:
            raise ValueError(f"Unsupported extraction strategy: {request.strategy}")

        extractor = self._snippet_extractors[request.strategy]
        snippets = await extractor.extract(request.file_path, language)

        # Domain logic: filter out empty snippets
        filtered_snippets = [snippet for snippet in snippets if snippet.strip()]

        return SnippetExtractionResult(snippets=filtered_snippets, language=language)

    def _sanitize_uri(self, uri_or_path_like: str) -> AnyUrl:
        """Convert a URI or path-like string to a URI."""
        # If it's git-clonable, it's valid
        if is_valid_clone_target(uri_or_path_like):
            return domain_entities.WorkingCopy.sanitize_git_url(uri_or_path_like)
        # If it's a local directory, it's valid
        if Path(uri_or_path_like).is_dir():
            return domain_entities.WorkingCopy.sanitize_local_path(uri_or_path_like)
        raise ValueError(f"Unsupported source: {uri_or_path_like}")
