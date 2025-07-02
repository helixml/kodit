"""Factories for creating snippet query providers."""

from pathlib import Path

from kodit.infrastructure.snippet_extraction.snippet_query_provider import (
    FileSystemSnippetQueryProvider,
    SnippetQueryProvider,
)


def create_snippet_query_provider() -> SnippetQueryProvider:
    """Create a snippet query provider."""
    return FileSystemSnippetQueryProvider(Path(__file__).parent / "languages")
