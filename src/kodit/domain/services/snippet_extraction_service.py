"""Snippet extraction service."""

from abc import ABC, abstractmethod
from pathlib import Path

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
