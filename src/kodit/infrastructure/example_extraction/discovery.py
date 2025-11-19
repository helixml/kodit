"""Example file discovery from repositories."""

from pathlib import Path
from typing import ClassVar


class ExampleDiscovery:
    """Discovers example files in repositories."""

    EXAMPLE_DIRECTORIES: ClassVar[set[str]] = {
        "examples",
        "example",
        "samples",
        "sample",
        "demos",
        "demo",
        "tutorials",
        "tutorial",
    }

    DOCUMENTATION_EXTENSIONS: ClassVar[set[str]] = {
        ".md",
        ".markdown",
        ".rst",
        ".adoc",
        ".asciidoc",
    }

    def is_example_directory_file(self, file_path: str) -> bool:
        """Check if file is in an example directory."""
        path = Path(file_path)
        parts = [p.lower() for p in path.parts]
        return any(part in self.EXAMPLE_DIRECTORIES for part in parts)

    def is_documentation_file(self, file_path: str) -> bool:
        """Check if file is a documentation file."""
        return Path(file_path).suffix.lower() in self.DOCUMENTATION_EXTENSIONS

    def is_example_candidate(self, file_path: str) -> bool:
        """Check if file should be processed for examples."""
        return self.is_example_directory_file(file_path) or self.is_documentation_file(
            file_path
        )
