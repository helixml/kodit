import os
from pathlib import Path


class QueryProvider:
    """Provides tree-sitter queries from .scm files."""

    def __init__(self, query_path: Path | None = None) -> None:
        """Initialize the query provider."""
        if query_path:
            self.query_path = query_path
        else:
            # Default path relative to this file
            self.query_path = Path(os.path.dirname(__file__)) / "queries"

    def get_query(self, language: str) -> str:
        """Get the query for a given language.

        Args:
            language: The programming language

        Returns:
            The query string

        Raises:
            FileNotFoundError: If the query file for the language is not found

        """
        query_file = self.query_path / f"{language}.scm"
        if not query_file.exists():
            raise FileNotFoundError(f"Query file not found for language: {language}")

        return query_file.read_text()
