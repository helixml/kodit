"""Factory for creating snippet extraction services."""

from pathlib import Path

from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.enums import SnippetExtractionStrategy
from kodit.domain.repositories import FileRepository, SnippetRepository
from kodit.domain.services.snippet_extraction_service import (
    SnippetExtractionDomainService,
)
from kodit.infrastructure.snippet_extraction.language_detection_service import (
    FileSystemLanguageDetectionService,
)
from kodit.infrastructure.snippet_extraction.snippet_query_provider import (
    FileSystemSnippetQueryProvider,
)
from kodit.infrastructure.snippet_extraction.tree_sitter_snippet_extractor import (
    TreeSitterSnippetExtractor,
)
from kodit.infrastructure.sqlalchemy.file_repository import SqlAlchemyFileRepository
from kodit.infrastructure.sqlalchemy.snippet_repository import (
    SqlAlchemySnippetRepository,
)


def create_snippet_extraction_domain_service() -> SnippetExtractionDomainService:
    """Create a snippet extraction domain service with all dependencies.

    Returns:
        Configured snippet extraction domain service

    """
    # Language mapping from the existing languages module
    language_map = {
        # JavaScript/TypeScript
        "js": "javascript",
        "jsx": "javascript",
        "ts": "typescript",
        "tsx": "typescript",
        # Python
        "py": "python",
        # Rust
        "rs": "rust",
        # Go
        "go": "go",
        # C/C++
        "cpp": "cpp",
        "hpp": "cpp",
        "c": "c",
        "h": "c",
        # C#
        "cs": "csharp",
        # Ruby
        "rb": "ruby",
        # Java
        "java": "java",
        # PHP
        "php": "php",
        # Swift
        "swift": "swift",
        # Kotlin
        "kt": "kotlin",
    }

    # Create infrastructure services
    language_detector = FileSystemLanguageDetectionService(language_map)
    query_provider = FileSystemSnippetQueryProvider(Path(__file__).parent / "languages")

    # Create snippet extractors
    method_extractor = TreeSitterSnippetExtractor(query_provider)

    snippet_extractors = {
        SnippetExtractionStrategy.METHOD_BASED: method_extractor,
    }

    # Create domain service
    return SnippetExtractionDomainService(language_detector, snippet_extractors)


def create_snippet_repositories(
    session: AsyncSession,
) -> tuple[SnippetRepository, FileRepository]:
    """Create snippet and file repositories.

    Args:
        session: SQLAlchemy session

    Returns:
        Tuple of (snippet_repository, file_repository)

    """
    snippet_repository = SqlAlchemySnippetRepository(session)
    file_repository = SqlAlchemyFileRepository(session)
    return snippet_repository, file_repository
