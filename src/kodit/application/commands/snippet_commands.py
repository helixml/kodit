"""Application commands for snippet operations."""

from dataclasses import dataclass
from pathlib import Path

from kodit.domain.enums import SnippetExtractionStrategy


@dataclass
class ExtractSnippetsCommand:
    """Application command for extracting snippets from files."""

    file_path: Path
    strategy: SnippetExtractionStrategy = SnippetExtractionStrategy.METHOD_BASED


@dataclass
class CreateIndexSnippetsCommand:
    """Application command for creating snippets for an entire index."""

    index_id: int
    strategy: SnippetExtractionStrategy = SnippetExtractionStrategy.METHOD_BASED
