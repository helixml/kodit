"""Pure domain value objects and DTOs."""

from dataclasses import dataclass
from enum import Enum, IntEnum
from pathlib import Path

from pydantic import BaseModel

from kodit.domain.enums import SnippetExtractionStrategy


class SourceType(IntEnum):
    """The type of source."""

    UNKNOWN = 0
    FOLDER = 1
    GIT = 2


class SnippetContentType(IntEnum):
    """Type of snippet content."""

    UNKNOWN = 0
    ORIGINAL = 1
    SUMMARY = 2


class SnippetContent(BaseModel):
    """Snippet content domain value object."""

    type: SnippetContentType
    value: str
    language: str


class SearchType(Enum):
    """Type of search to perform."""

    BM25 = "bm25"
    VECTOR = "vector"
    HYBRID = "hybrid"


@dataclass(frozen=True)
class SnippetExtractionRequest:
    """Domain value object for snippet extraction request."""

    file_path: Path
    strategy: SnippetExtractionStrategy = SnippetExtractionStrategy.METHOD_BASED


@dataclass(frozen=True)
class SnippetExtractionResult:
    """Domain value object for snippet extraction result."""

    snippets: list[str]
    language: str


@dataclass(frozen=True)
class Document:
    """Generic document value object for indexing."""

    snippet_id: int
    text: str


@dataclass(frozen=True)
class DocumentSearchResult:
    """Generic document search result value object."""

    snippet_id: int
    score: float


@dataclass(frozen=True)
class SearchResult:
    """Generic search result value object."""

    snippet_id: int
    score: float


@dataclass(frozen=True)
class IndexRequest:
    """Generic indexing request value object."""

    documents: list[Document]


@dataclass(frozen=True)
class SearchRequest:
    """Generic search request value object."""

    query: str
    top_k: int = 10
    snippet_ids: list[int] | None = None


@dataclass(frozen=True)
class DeleteRequest:
    """Generic deletion request value object."""

    snippet_ids: list[int]


@dataclass(frozen=True)
class IndexResult:
    """Generic indexing result value object."""

    snippet_id: int


@dataclass(frozen=True)
class SnippetSearchFilters:
    """Value object for filtering snippet search results."""

    language: str | None = None
    author: str | None = None
    file_extension: str | None = None
    max_results: int = 100


class SnippetQuery(BaseModel):
    """Domain query object for snippet searches."""

    text: str
    search_type: SearchType = SearchType.HYBRID
    filters: SnippetSearchFilters = SnippetSearchFilters()
    top_k: int = 10

    class Config:
        """Pydantic model configuration."""

        frozen = True


class SnippetSearchResult(BaseModel):
    """Domain result object for snippet searches."""

    snippet_id: int
    content: str
    summary: str
    score: float
    file_path: Path
    language: str | None = None
    authors: list[str] = []

    class Config:
        """Pydantic model configuration."""

        frozen = True


@dataclass(frozen=True)
class LanguageExtensions:
    """Value object for language to file extension mappings."""

    language: str
    extensions: list[str]

    @classmethod
    def get_supported_languages(cls) -> list[str]:
        """Get all supported programming languages."""
        return [
            "python",
            "javascript",
            "typescript",
            "java",
            "c",
            "cpp",
            "csharp",
            "go",
            "rust",
            "php",
            "ruby",
            "swift",
            "kotlin",
            "scala",
            "r",
            "sql",
            "html",
            "css",
            "json",
            "yaml",
            "xml",
            "markdown",
            "shell",
        ]

    @classmethod
    def get_extensions_for_language(cls, language: str) -> list[str]:
        """Get file extensions for a given language."""
        language_map = {
            "python": [".py", ".pyw", ".pyi"],
            "javascript": [".js", ".jsx", ".mjs"],
            "typescript": [".ts", ".tsx"],
            "java": [".java"],
            "c": [".c", ".h"],
            "cpp": [".cpp", ".cc", ".cxx", ".hpp", ".hxx"],
            "csharp": [".cs"],
            "go": [".go"],
            "rust": [".rs"],
            "php": [".php"],
            "ruby": [".rb"],
            "swift": [".swift"],
            "kotlin": [".kt", ".kts"],
            "scala": [".scala", ".sc"],
            "r": [".r", ".R"],
            "sql": [".sql"],
            "html": [".html", ".htm"],
            "css": [".css", ".scss", ".sass", ".less"],
            "json": [".json"],
            "yaml": [".yaml", ".yml"],
            "xml": [".xml"],
            "markdown": [".md", ".markdown"],
            "shell": [".sh", ".bash", ".zsh", ".fish"],
        }
        return language_map.get(language.lower(), [])

    @classmethod
    def is_supported_language(cls, language: str) -> bool:
        """Check if a language is supported."""
        return language.lower() in cls.get_supported_languages()

    @classmethod
    def get_extensions_or_fallback(cls, language: str) -> list[str]:
        """Get extensions for language or return language as extension if not found."""
        language_lower = language.lower()
        if cls.is_supported_language(language_lower):
            return cls.get_extensions_for_language(language_lower)
        return [language_lower]
