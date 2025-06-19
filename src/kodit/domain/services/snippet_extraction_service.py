"""Domain services for snippet extraction."""

from abc import ABC, abstractmethod
from collections.abc import Mapping
from pathlib import Path

from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.value_objects import SnippetExtractionRequest, SnippetExtractionResult


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


class SnippetExtractionService(ABC):
    """Domain service for extracting snippets from source code."""

    @abstractmethod
    async def extract_snippets(
        self, request: SnippetExtractionRequest
    ) -> SnippetExtractionResult:
        """Extract snippets from a file using the specified strategy."""


class SnippetExtractionDomainService:
    """Domain service implementation for snippet extraction business logic."""

    def __init__(
        self,
        language_detector: LanguageDetectionService,
        snippet_extractors: Mapping[SnippetExtractionStrategy, SnippetExtractor],
    ) -> None:
        """Initialize the snippet extraction domain service.

        Args:
            language_detector: Service for detecting programming languages
            snippet_extractors: Dictionary mapping strategies to extractor
                implementations

        """
        self.language_detector = language_detector
        self.snippet_extractors = snippet_extractors

    async def extract_snippets(
        self, request: SnippetExtractionRequest
    ) -> SnippetExtractionResult:
        """Extract snippets from a file using the specified strategy.

        Args:
            request: The snippet extraction request

        Returns:
            SnippetExtractionResult containing the extracted snippets and
            detected language

        Raises:
            ValueError: If the file doesn't exist or strategy is unsupported

        """
        # Domain logic: validate file exists
        if not request.file_path.exists():
            raise ValueError(f"File does not exist: {request.file_path}")

        # Domain logic: detect language
        language = await self.language_detector.detect_language(request.file_path)

        # Domain logic: choose strategy and extractor
        if request.strategy not in self.snippet_extractors:
            raise ValueError(f"Unsupported extraction strategy: {request.strategy}")

        extractor = self.snippet_extractors[request.strategy]
        snippets = await extractor.extract(request.file_path, language)

        # Domain logic: filter out empty snippets
        filtered_snippets = [snippet for snippet in snippets if snippet.strip()]

        return SnippetExtractionResult(snippets=filtered_snippets, language=language)
