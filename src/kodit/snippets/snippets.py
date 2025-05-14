"""Generate snippets from a file."""

from dataclasses import dataclass
from pathlib import Path

from kodit.snippets.languages import detect_language
from kodit.snippets.method_snippets import MethodSnippets


@dataclass
class Snippet:
    """A snippet of code."""

    text: str


class SnippetService:
    """Factory for generating snippets from a file.

    This is required because there's going to be multiple ways to generate snippets.
    """

    def __init__(self) -> None:
        """Initialize the snippet factory."""
        self.language_dir = Path(__file__).parent / "languages"

    def snippets_for_file(self, file_path: Path) -> list[Snippet]:
        """Generate snippets from a file."""
        language = detect_language(file_path)

        try:
            query_path = self.language_dir / f"{language}.scm"
            with query_path.open() as f:
                query = f.read()
        except Exception as e:
            msg = f"Unsupported language: {file_path}"
            raise ValueError(msg) from e

        method_analser = MethodSnippets(language, query)

        try:
            file_bytes = file_path.read_bytes()
        except Exception as e:
            msg = f"Failed to read file: {file_path}"
            raise ValueError(msg) from e

        method_snippets = method_analser.extract(file_bytes)
        return [Snippet(text=snippet) for snippet in method_snippets]
