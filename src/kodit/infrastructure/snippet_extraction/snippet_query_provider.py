"""Infrastructure implementation for loading snippet queries from files."""

from abc import ABC, abstractmethod
from pathlib import Path


class SnippetQueryProvider(ABC):
    """Abstract interface for providing snippet queries."""

    @abstractmethod
    async def get_query(self, language: str) -> str:
        """Get the query for a specific language."""


class FileSystemSnippetQueryProvider(SnippetQueryProvider):
    """Infrastructure implementation for loading snippet queries from files."""

    def __init__(self, query_directory: Path) -> None:
        """Initialize the query provider.

        Args:
            query_directory: Directory containing query files

        """
        self.query_directory = query_directory

    async def get_query(self, language: str) -> str:
        """Load query from file system.

        Args:
            language: The programming language to get the query for

        Returns:
            The query string for the language

        Raises:
            FileNotFoundError: If the query file doesn't exist

        """
        query_path = self.query_directory / f"{language}.scm"

        if not query_path.exists():
            raise FileNotFoundError(f"Query file not found: {query_path}")

        return query_path.read_text()
