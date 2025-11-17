"""Template-based API documentation formatter using Jinja2."""

import re
from pathlib import Path

import structlog
from jinja2 import Environment, FileSystemLoader

from kodit.infrastructure.slicing.code_elements import ModuleDefinition, ParsedFile


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
        # Enrich modules with extracted signatures if needed for Python
        if self.language == "python":
            modules = self._enrich_python_signatures(modules)

        return self.template.render(
            modules=modules,
            language=language,
        )

    def _enrich_python_signatures(
        self, modules: list[ModuleDefinition]
    ) -> list[ModuleDefinition]:
        """Enrich Python modules with extracted method signatures.

        Args:
            modules: Original module definitions

        Returns:
            Modules with enriched parameter lists

        """
        for module in modules:
            for cls in module.classes:
                for method in cls.methods:
                    if not method.parameters:
                        # Extract signature from node if parameters are missing
                        parsed_file = self._find_parsed_file(module, method.file)
                        if parsed_file:
                            sig = self._extract_signature(parsed_file, method.node)
                            params = self._parse_python_params(sig)
                            # Create new method with updated parameters
                            method.parameters.extend(params)

        return modules

    def _find_parsed_file(
        self, module: ModuleDefinition, file_path: Path
    ) -> ParsedFile | None:
        """Find parsed file in module."""
        for parsed in module.files:
            if parsed.path == file_path:
                return parsed
        return None

    def _extract_signature(self, parsed_file: object, node: object) -> str:
        """Extract signature from tree-sitter node."""
        if not hasattr(node, "start_byte") or not hasattr(node, "end_byte"):
            return ""

        start = node.start_byte  # type: ignore[attr-defined]
        end = node.end_byte  # type: ignore[attr-defined]

        try:
            source = parsed_file.source_code[start:end].decode("utf-8")  # type: ignore[attr-defined]
            # Extract just the signature (up to the colon)
            lines = []
            for line in source.split("\n"):
                lines.append(line)
                if ":" in line and line.count("(") == line.count(")"):
                    break
            return "\n".join(lines)
        except (UnicodeDecodeError, IndexError, AttributeError):
            return ""

    def _parse_python_params(self, signature: str) -> list[str]:
        """Parse parameters from Python function signature.

        Args:
            signature: Function signature string

        Returns:
            List of parameter strings with types

        """
        # Extract params from signature like "def add(self, a: float, b: float)"

        # Find content between parentheses
        match = re.search(r"\((.*?)\)", signature, re.DOTALL)
        if not match:
            return []

        params_str = match.group(1).strip()
        if not params_str:
            return []

        # Split by commas (simple approach, doesn't handle nested structures)
        return [p.strip() for p in params_str.split(",")]
