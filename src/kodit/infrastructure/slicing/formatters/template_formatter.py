"""Template-based API documentation formatter using Jinja2."""

import re
from pathlib import Path

import structlog
from jinja2 import Environment, FileSystemLoader

from kodit.infrastructure.slicing.code_elements import ModuleDefinition


def regex_replace(value: str, pattern: str, replacement: str = "") -> str:
    """Jinja2 filter to replace using regex."""
    return re.sub(pattern, replacement, value)


def dedent_filter(value: str) -> str:
    """Jinja2 filter to remove common leading whitespace from docstrings."""
    # First, split into lines
    lines = value.splitlines()
    if not lines:
        return ""

    # Find the minimum indentation (ignoring the first line and empty lines)
    # This matches Python's inspect.cleandoc() behavior
    indents = []
    for i, line in enumerate(lines):
        if i == 0:  # Skip first line
            continue
        stripped = line.lstrip()
        if stripped:  # Only consider non-empty lines
            indents.append(len(line) - len(stripped))

    if not indents:
        return value.strip()

    # Remove the minimum indentation from all lines except the first
    min_indent = min(indents)
    dedented_lines = [lines[0]]  # Keep first line as-is
    for line in lines[1:]:
        if line.strip():  # Non-empty line
            dedented_lines.append(line[min_indent:])
        else:  # Empty line
            dedented_lines.append("")

    return "\n".join(dedented_lines).strip()


def regex_match(value: str, pattern: str, attribute: str | None = None) -> bool:
    """Jinja2 test to check if value matches regex pattern.

    Args:
        value: The value to test (or object if attribute is specified)
        pattern: The regex pattern to match
        attribute: Optional attribute name to extract from value first

    Returns:
        True if the value matches the pattern

    """
    if attribute:
        value = getattr(value, attribute, "")
    return bool(re.match(pattern, str(value)))


class TemplateAPIDocFormatter:
    """Formats code into API documentation using Jinja2 templates."""

    def __init__(self, language: str) -> None:
        """Initialize formatter with language-specific template.

        Args:
            language: Programming language (e.g., 'python', 'go', 'java')

        """
        self.log = structlog.get_logger(__name__)
        self.language = language.lower()

        # Set up Jinja2 environment
        template_dir = Path(__file__).parent / "templates"
        self.env = Environment(
            loader=FileSystemLoader(str(template_dir)),
            autoescape=False,  # Markdown output, not HTML  # noqa: S701
            trim_blocks=True,
            lstrip_blocks=True,
        )

        # Add custom filters and tests BEFORE loading templates
        self.env.filters["regex_replace"] = regex_replace
        self.env.filters["dedent"] = dedent_filter
        self.env.tests["match"] = regex_match

        # Load language-specific template
        template_name = f"{self.language}.md.j2"
        self.template = self.env.get_template(template_name)

    def format_combined_markdown(
        self,
        modules: list[ModuleDefinition],
        language: str,
    ) -> str:
        """Generate API documentation markdown from modules.

        Args:
            modules: List of module definitions to document
            language: Programming language for display

        Returns:
            Formatted markdown documentation

        """
        return self.template.render(
            modules=modules,
            language=language,
        )
