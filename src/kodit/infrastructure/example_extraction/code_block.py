"""Code block extraction from documentation."""

from dataclasses import dataclass


@dataclass(frozen=True)
class CodeBlock:
    """Represents a code block extracted from documentation."""

    content: str
    language: str | None
    line_start: int
    line_end: int
    context: str | None = None
