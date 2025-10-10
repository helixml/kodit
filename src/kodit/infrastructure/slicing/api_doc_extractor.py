"""API documentation extractor."""

import structlog

from kodit.domain.enrichments.usage.api_docs import APIDocEnrichment
from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.ast_analyzer import (
    ASTAnalyzer,
    ClassDefinition,
    FunctionDefinition,
    ModuleDefinition,
    ParsedFile,
    TypeDefinition,
)


class APIDocExtractor:
    """Extract API documentation from code files."""

    # Languages that should have API docs generated
    SUPPORTED_LANGUAGES = frozenset(
        {
            "c",
            "cpp",
            "csharp",
            "go",
            "java",
            "javascript",
            "python",
            "rust",
        }
    )

    def __init__(self) -> None:
        """Initialize the API doc extractor."""
        self.log = structlog.get_logger(__name__)

    def extract_api_docs(
        self,
        files: list[GitFile],
        language: str,
        *,
        include_private: bool = False,
    ) -> list[APIDocEnrichment]:
        """Extract API documentation enrichments from files."""
        if not files:
            return []

        # Filter out languages that shouldn't have API docs
        if language not in self.SUPPORTED_LANGUAGES:
            self.log.debug(
                "Language not supported for API docs", language=language
            )
            return []

        try:
            analyzer = ASTAnalyzer(language)
            parsed_files = analyzer.parse_files(files)
            modules = analyzer.extract_module_definitions(
                parsed_files, include_private=include_private
            )
        except ValueError:
            self.log.debug("Unsupported language", language=language)
            return []

        enrichments = []
        for module in modules:
            if not self._has_content(module):
                continue

            markdown_content = self._generate_markdown(module)
            enrichment = APIDocEnrichment(
                entity_id=module.files[0].git_file.blob_sha,
                module_path=module.module_path,
                content=markdown_content,
            )
            enrichments.append(enrichment)

        return enrichments

    def _has_content(self, module: ModuleDefinition) -> bool:
        """Check if module has any API elements."""
        return bool(
            module.functions or module.classes or module.types or module.constants
        )

    def _generate_markdown(self, module: ModuleDefinition) -> str:  # noqa: C901
        """Generate Go-Doc style Markdown for a module."""
        lines = []

        # Header
        lines.append(f"# package {module.module_path}")
        lines.append("")

        # Overview section (module docstring)
        if module.module_docstring:
            lines.append("## Overview")
            lines.append("")
            lines.append(module.module_docstring)
            lines.append("")

        # Index
        if self._should_generate_index(module):
            lines.extend(self._generate_index(module))
            lines.append("")

        # Constants
        if module.constants:
            lines.append("## Constants")
            lines.append("")
            for _name, node in module.constants:
                parsed_file = self._find_parsed_file(module, node)
                if parsed_file:
                    signature = self._extract_source(parsed_file, node)
                    lines.append("```")
                    lines.append(signature.strip())
                    lines.append("```")
                    lines.append("")

        # Functions
        if module.functions:
            lines.append("## Functions")
            lines.append("")
            for func in sorted(module.functions, key=lambda f: f.simple_name):
                lines.extend(self._format_function(func, module))

        # Types
        if module.types:
            lines.append("## Types")
            lines.append("")
            for typ in sorted(module.types, key=lambda t: t.simple_name):
                lines.extend(self._format_type(typ, module))

        if module.classes:
            if not module.types:
                lines.append("## Types")
                lines.append("")
            for cls in sorted(module.classes, key=lambda c: c.simple_name):
                lines.extend(self._format_class(cls, module))

        # Source Files
        lines.append("## Source Files")
        lines.append("")
        lines.extend(f"- {parsed.git_file.path}" for parsed in module.files)
        lines.append("")

        return "\n".join(lines)

    def _should_generate_index(self, module: ModuleDefinition) -> bool:
        """Check if we should generate an index."""
        total_items = (
            len(module.constants)
            + len(module.functions)
            + len(module.types)
            + len(module.classes)
        )
        return total_items > 3

    def _generate_index(self, module: ModuleDefinition) -> list[str]:
        """Generate an index of all public items."""
        lines = ["## Index", ""]

        if module.constants:
            lines.append("### Constants")
            for name, _ in sorted(module.constants, key=lambda c: c[0]):
                lines.append(f"- `{name}`")
            lines.append("")

        if module.functions:
            lines.append("### Functions")
            for func in sorted(module.functions, key=lambda f: f.simple_name):
                sig = self._generate_function_signature_short(func)
                lines.append(f"- `{sig}`")
            lines.append("")

        if module.types or module.classes:
            lines.append("### Types")
            lines.extend(
                f"- `type {typ.simple_name}`"
                for typ in sorted(module.types, key=lambda t: t.simple_name)
            )
            lines.extend(
                f"- `type {cls.simple_name}`"
                for cls in sorted(module.classes, key=lambda c: c.simple_name)
            )
            lines.append("")

        return lines

    def _generate_function_signature_short(self, func: FunctionDefinition) -> str:
        """Generate short function signature for index."""
        params = ", ".join(func.parameters) if func.parameters else "..."
        ret = f" -> {func.return_type}" if func.return_type else ""
        return f"{func.simple_name}({params}){ret}"

    def _format_function(
        self, func: FunctionDefinition, module: ModuleDefinition
    ) -> list[str]:
        """Format a function in Go-Doc style."""
        lines = [f"### func {func.simple_name}", ""]

        # Signature
        parsed_file = self._find_parsed_file(module, func.node)
        if parsed_file:
            signature = self._extract_source(parsed_file, func.node)
            lines.append("```")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        # Documentation
        if func.docstring:
            lines.append(func.docstring)
            lines.append("")

        return lines

    def _format_type(
        self, typ: TypeDefinition, module: ModuleDefinition
    ) -> list[str]:
        """Format a type in Go-Doc style."""
        lines = [f"### type {typ.simple_name}", ""]

        # Signature
        parsed_file = self._find_parsed_file(module, typ.node)
        if parsed_file:
            signature = self._extract_source(parsed_file, typ.node)
            lines.append("```")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        # Documentation
        if typ.docstring:
            lines.append(typ.docstring)
            lines.append("")

        return lines

    def _format_class(
        self, cls: ClassDefinition, module: ModuleDefinition
    ) -> list[str]:
        """Format a class in Go-Doc style."""
        lines = [f"### type {cls.simple_name}", ""]

        # Class signature
        parsed_file = self._find_parsed_file(module, cls.node)
        if parsed_file:
            signature = self._extract_source(parsed_file, cls.node)
            lines.append("```")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        # Class documentation
        if cls.docstring:
            lines.append(cls.docstring)
            lines.append("")

        # Methods
        if cls.methods:
            for method in sorted(cls.methods, key=lambda m: m.simple_name):
                lines.extend(self._format_method(method, cls, module))

        return lines

    def _format_method(
        self,
        method: FunctionDefinition,
        cls: ClassDefinition,
        module: ModuleDefinition,
    ) -> list[str]:
        """Format a method in Go-Doc style."""
        lines = [f"#### func ({cls.simple_name}) {method.simple_name}", ""]

        # Method signature
        parsed_file = self._find_parsed_file(module, method.node)
        if parsed_file:
            signature = self._extract_source(parsed_file, method.node)
            lines.append("```")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        # Method documentation
        if method.docstring:
            lines.append(method.docstring)
            lines.append("")

        return lines

    def _find_parsed_file(
        self, module: ModuleDefinition, node: object
    ) -> ParsedFile | None:
        """Find the parsed file containing a given node."""
        for parsed in module.files:
            if hasattr(node, "tree") and node.tree == parsed.tree:  # type: ignore[attr-defined]
                return parsed
        return module.files[0] if module.files else None

    def _extract_source(self, parsed_file: ParsedFile, node: object) -> str:
        """Extract source code for a node."""
        if not hasattr(node, "start_byte") or not hasattr(node, "end_byte"):
            return "<source unavailable>"

        start = node.start_byte  # type: ignore[attr-defined]
        end = node.end_byte  # type: ignore[attr-defined]

        try:
            source = parsed_file.source_code[start:end].decode("utf-8")
            # Extract just the signature
            return self._extract_signature_only(source)
        except (UnicodeDecodeError, IndexError):
            return "<source unavailable>"

    def _extract_signature_only(self, source: str) -> str:
        """Extract just the signature from a definition.

        This removes function bodies and only keeps the declaration/signature.
        """
        lines = source.split("\n")
        signature_lines = []

        for line in lines:
            # Stop at the first line that ends a signature
            signature_lines.append(line)

            # Check for end of signature markers
            stripped = line.strip()

            # Python: colon ends signature (unless inside brackets)
            if ":" in line:
                open_parens = line.count("(") - line.count(")")
                open_brackets = line.count("[") - line.count("]")
                open_braces = line.count("{") - line.count("}")
                if open_parens == 0 and open_brackets == 0 and open_braces == 0:
                    break

            # Go/Java/C/C++/Rust/JS: opening brace often starts body
            if stripped.endswith("{"):
                # Remove the opening brace for cleaner signatures
                signature_lines[-1] = line.rstrip("{").rstrip()
                break

            # Go: if signature ends without brace on same line
            if stripped.endswith(")") and not any(c in line for c in ["{", ":"]):
                # Might be complete - check if next line exists
                continue

        return "\n".join(signature_lines)
