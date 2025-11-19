"""Documentation parser protocol and implementations."""

import re
from abc import ABC, abstractmethod

from kodit.infrastructure.example_extraction.code_block import CodeBlock


class DocumentationParser(ABC):
    """Protocol for parsing code blocks from documentation."""

    @abstractmethod
    def parse(self, content: str) -> list[CodeBlock]:
        """Extract code blocks from documentation content."""


class MarkdownParser(DocumentationParser):
    """Parser for Markdown documentation."""

    def parse(self, content: str) -> list[CodeBlock]:
        """Extract code blocks from Markdown content."""
        blocks: list[CodeBlock] = []
        lines = content.split("\n")
        i = 0

        while i < len(lines):
            line = lines[i]
            match = re.match(r"^```(\w+)?", line)
            if match:
                language = match.group(1)
                line_start = i + 1
                code_lines = []
                i += 1

                while i < len(lines) and not lines[i].startswith("```"):
                    code_lines.append(lines[i])
                    i += 1

                if code_lines:
                    context = self._find_context(lines, line_start - 1)
                    blocks.append(
                        CodeBlock(
                            content="\n".join(code_lines),
                            language=language,
                            line_start=line_start,
                            line_end=i - 1,
                            context=context,
                        )
                    )
            i += 1

        return blocks

    def _find_context(self, lines: list[str], block_line: int) -> str | None:
        """Find heading or preceding paragraph near code block."""
        heading = None
        for i in range(max(0, block_line - 10), block_line):
            if lines[i].startswith("#"):
                heading = lines[i].lstrip("#").strip()

        if heading:
            return heading

        paragraph_lines = []
        for i in range(max(0, block_line - 3), block_line):
            stripped = lines[i].strip()
            if stripped and not stripped.startswith("#"):
                paragraph_lines.append(stripped)

        return " ".join(paragraph_lines) if paragraph_lines else None


class RstParser(DocumentationParser):
    """Parser for reStructuredText documentation."""

    def parse(self, content: str) -> list[CodeBlock]:
        """Extract code blocks from RST content."""
        blocks: list[CodeBlock] = []
        lines = content.split("\n")
        i = 0

        while i < len(lines):
            line = lines[i]
            match = re.match(r"^\.\.\s+(code-block|code)::\s*(\w+)?", line)
            if match:
                language = match.group(2)
                i += 1

                while i < len(lines) and not lines[i].strip():
                    i += 1

                if i >= len(lines):
                    break

                base_indent = len(lines[i]) - len(lines[i].lstrip())
                line_start = i
                code_lines = []

                while i < len(lines):
                    current_line = lines[i]
                    if not current_line.strip():
                        i += 1
                        continue
                    current_indent = len(current_line) - len(current_line.lstrip())
                    if current_indent < base_indent:
                        break
                    code_lines.append(current_line[base_indent:])
                    i += 1

                if code_lines:
                    blocks.append(
                        CodeBlock(
                            content="\n".join(code_lines),
                            language=language,
                            line_start=line_start,
                            line_end=i - 1,
                            context=None,
                        )
                    )
            else:
                i += 1

        return blocks


class ParserFactory:
    """Factory for creating documentation parsers."""

    @staticmethod
    def create(file_extension: str) -> DocumentationParser | None:
        """Create parser based on file extension."""
        extension = file_extension.lower()
        if extension in {".md", ".markdown"}:
            return MarkdownParser()
        if extension in {".rst"}:
            return RstParser()
        return None
