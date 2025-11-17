"""C++ API documentation formatter."""

import re

import structlog

from kodit.infrastructure.slicing.code_elements import (
    ClassDefinition,
    FunctionDefinition,
    ModuleDefinition,
    ParsedFile,
    TypeDefinition,
)


class CppAPIDocFormatter:
    """Formats C++ code into API documentation markdown."""

    def __init__(self) -> None:
        """Initialize the formatter."""
        self.log = structlog.get_logger(__name__)
        self._code_fence = "cpp"

    def format_combined_markdown(
        self,
        modules: list[ModuleDefinition],
        language: str,
    ) -> str:
        """Generate C++-style markdown for all modules combined."""
        lines = []

        lines.append(f"## {language} Index")
        lines.append("")
        lines.extend(
            f"- [{module.module_path}](#{self._anchor(module.module_path)})"
            for module in sorted(modules, key=lambda m: m.module_path)
        )
        lines.append("")

        for module in sorted(modules, key=lambda m: m.module_path):
            lines.extend(self._format_module_section(module))

        return "\n".join(lines)

    def _format_module_section(self, module: ModuleDefinition) -> list[str]:
        """Generate markdown section for a single module."""
        lines = []

        lines.append(f"## {module.module_path}")
        lines.append("")
        if module.module_docstring:
            lines.append(module.module_docstring)
            lines.append("")

        lines.extend(self._format_constants_section(module))
        lines.extend(self._format_types_section(module))
        lines.extend(self._format_functions_section(module))
        lines.extend(self._format_source_files_section(module))

        return lines

    def _format_constants_section(self, module: ModuleDefinition) -> list[str]:
        """Format constants section for a module."""
        if not module.constants:
            return []

        lines = ["### Constants", ""]
        for _name, node in module.constants:
            parsed_file = self._find_parsed_file(module, node)
            if parsed_file:
                signature = self._extract_source(parsed_file, node)
                lines.append(f"```{self._code_fence}")
                lines.append(signature.strip())
                lines.append("```")
                lines.append("")
        return lines

    def _format_functions_section(self, module: ModuleDefinition) -> list[str]:
        """Format functions section for a module."""
        if not module.functions:
            return []

        valid_functions = [
            f for f in module.functions if self._is_valid_function_name(f.simple_name)
        ]

        if not valid_functions:
            return []

        lines = ["### Functions", ""]
        for func in sorted(valid_functions, key=lambda f: f.simple_name):
            lines.extend(self._format_function_standalone(func, module))
        return lines

    def _format_types_section(self, module: ModuleDefinition) -> list[str]:
        """Format types section for a module."""
        if not (module.types or module.classes):
            return []

        lines = ["### Types", ""]

        for typ in sorted(module.types, key=lambda t: t.simple_name):
            lines.extend(self._format_type(typ, module))

        for cls in sorted(module.classes, key=lambda c: c.simple_name):
            lines.extend(self._format_class(cls, module))

        return lines

    def _format_source_files_section(self, module: ModuleDefinition) -> list[str]:
        """Format source files section for a module."""
        from pathlib import Path

        non_init_files = [
            parsed
            for parsed in module.files
            if Path(parsed.git_file.path).name != "__init__.py"
        ]

        if not non_init_files:
            return []

        lines = ["### Source Files", ""]
        lines.extend(
            f"- `{parsed.git_file.path}`"
            for parsed in sorted(non_init_files, key=lambda f: f.git_file.path)
        )
        lines.append("")
        return lines

    def _format_function_standalone(
        self, func: FunctionDefinition, module: ModuleDefinition
    ) -> list[str]:
        """Format a standalone function."""
        lines = [f"#### {func.simple_name}", ""]

        parsed_file = self._find_parsed_file_for_function(module, func)
        if parsed_file:
            signature = self._extract_source(parsed_file, func.node)
            lines.append(f"```{self._code_fence}")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        if func.docstring:
            lines.append(func.docstring)
            lines.append("")

        return lines

    def _format_type(self, typ: TypeDefinition, module: ModuleDefinition) -> list[str]:
        """Format a type definition."""
        lines = [f"#### {typ.simple_name}", ""]

        parsed_file = self._find_parsed_file_for_type(module, typ)
        if parsed_file:
            signature = self._extract_source(parsed_file, typ.node)
            lines.append(f"```{self._code_fence}")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        if typ.docstring:
            lines.append(typ.docstring)
            lines.append("")

        if typ.constructor_params:
            lines.append("**Fields:**")
            lines.append("")
            lines.extend(f"- `{param}`" for param in typ.constructor_params)
            lines.append("")

        return lines

    def _format_class(
        self, cls: ClassDefinition, module: ModuleDefinition
    ) -> list[str]:
        """Format a class definition."""
        lines = [f"#### {cls.simple_name}", ""]

        parsed_file = self._find_parsed_file_for_class(module, cls)
        if parsed_file:
            signature = self._extract_source(parsed_file, cls.node)
            lines.append(f"```{self._code_fence}")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        if cls.docstring:
            lines.append(cls.docstring)
            lines.append("")

        if cls.constructor_params:
            lines.append("**Constructor Parameters:**")
            lines.append("")
            lines.extend(f"- `{param}`" for param in cls.constructor_params)
            lines.append("")

        if cls.methods:
            valid_methods = [
                m for m in cls.methods if self._is_valid_function_name(m.simple_name)
            ]
            for method in sorted(valid_methods, key=lambda m: m.simple_name):
                lines.extend(self._format_method(method, cls, module))

        return lines

    def _format_method(
        self,
        method: FunctionDefinition,
        cls: ClassDefinition,
        module: ModuleDefinition,
    ) -> list[str]:
        """Format a method."""
        lines = [f"##### {cls.simple_name}::{method.simple_name}", ""]

        parsed_file = self._find_parsed_file_for_function(module, method)
        if parsed_file:
            signature = self._extract_source(parsed_file, method.node)
            lines.append(f"```{self._code_fence}")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        if method.docstring:
            lines.append(method.docstring)
            lines.append("")

        return lines

    def _anchor(self, text: str) -> str:
        """Generate markdown anchor from text."""
        anchor = text.lower()
        anchor = anchor.replace("/", "-").replace(".", "-")
        anchor = re.sub(r"[^a-z0-9\\-_]", "", anchor)
        anchor = re.sub(r"-+", "-", anchor)
        return anchor.strip("-")

    def _is_valid_function_name(self, name: str) -> bool:
        """Check if a function name should be included in API documentation."""
        if not name:
            return False
        if len(name) > 255:
            return False
        anonymous_patterns = ["anonymous", "default"]
        if name.lower() in anonymous_patterns:  # noqa: SIM103
            return False
        return True

    def _find_parsed_file_for_function(
        self, module: ModuleDefinition, func: FunctionDefinition
    ) -> ParsedFile | None:
        """Find the parsed file containing a function definition."""
        for parsed in module.files:
            if parsed.path == func.file:
                return parsed
        self.log.warning(
            "Could not find parsed file for function",
            module_path=module.module_path,
            function_file=str(func.file),
            file_count=len(module.files),
        )
        return None

    def _find_parsed_file_for_type(
        self, module: ModuleDefinition, typ: TypeDefinition
    ) -> ParsedFile | None:
        """Find the parsed file containing a type definition."""
        for parsed in module.files:
            if parsed.path == typ.file:
                return parsed
        self.log.warning(
            "Could not find parsed file for type",
            module_path=module.module_path,
            type_file=str(typ.file),
            file_count=len(module.files),
        )
        return None

    def _find_parsed_file_for_class(
        self, module: ModuleDefinition, cls: ClassDefinition
    ) -> ParsedFile | None:
        """Find the parsed file containing a class definition."""
        for parsed in module.files:
            if parsed.path == cls.file:
                return parsed
        self.log.warning(
            "Could not find parsed file for class",
            module_path=module.module_path,
            class_file=str(cls.file),
            file_count=len(module.files),
        )
        return None

    def _find_parsed_file(
        self, module: ModuleDefinition, node: object
    ) -> ParsedFile | None:
        """Find the parsed file containing a given node."""
        if hasattr(node, "tree"):
            node_tree = node.tree  # type: ignore[attr-defined]
            for parsed in module.files:
                if parsed.tree == node_tree:
                    return parsed

        self.log.warning(
            "Could not find parsed file for node",
            module_path=module.module_path,
            file_count=len(module.files),
        )
        return None

    def _extract_source(self, parsed_file: ParsedFile | None, node: object) -> str:
        """Extract source code for a node."""
        if not parsed_file:
            return "<source unavailable>"
        if not hasattr(node, "start_byte") or not hasattr(node, "end_byte"):
            return "<source unavailable>"
        start = node.start_byte  # type: ignore[attr-defined]
        end = node.end_byte  # type: ignore[attr-defined]
        try:
            source = parsed_file.source_code[start:end].decode("utf-8")
            return self._extract_signature_only(source)
        except (UnicodeDecodeError, IndexError):
            return "<source unavailable>"

    def _extract_signature_only(self, source: str) -> str:
        """Extract just the signature from a definition."""
        lines = source.split("\n")
        signature_lines = []

        for line in lines:
            signature_lines.append(line)
            stripped = line.strip()

            if stripped.endswith("{"):
                signature_lines[-1] = line.rstrip("{").rstrip()
                break

            if stripped.endswith(";"):
                break

        return "\n".join(signature_lines)
