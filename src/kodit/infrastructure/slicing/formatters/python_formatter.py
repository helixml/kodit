"""Python API documentation formatter in Pydoc-Markdown style."""

import re

import structlog

from kodit.infrastructure.slicing.code_elements import (
    ClassDefinition,
    FunctionDefinition,
    ModuleDefinition,
    ParsedFile,
    TypeDefinition,
)


class PythonAPIDocFormatter:
    """Formats Python code in Pydoc-Markdown style."""

    def __init__(self) -> None:
        """Initialize the formatter."""
        self.log = structlog.get_logger(__name__)

    def format_combined_markdown(
        self,
        modules: list[ModuleDefinition],
        language: str,
    ) -> str:
        """Generate Pydoc-Markdown style documentation for all modules."""
        lines = []

        # Generate index of all modules
        lines.append(f"# {language} API Reference")
        lines.append("")
        lines.extend(
            f"- [{module.module_path}](#{self._anchor(module.module_path)})"
            for module in sorted(modules, key=lambda m: m.module_path)
        )
        lines.append("")

        # Generate documentation for each module
        for module in sorted(modules, key=lambda m: m.module_path):
            lines.extend(self._format_module_section_python(module))

        return "\n".join(lines)

    def _format_module_section_python(self, module: ModuleDefinition) -> list[str]:
        """Generate markdown section for a single Python module."""
        lines = []

        # Module header and docstring
        lines.append(f"## {module.module_path}")
        lines.append("")

        if module.module_docstring:
            lines.append(module.module_docstring)
            lines.append("")

        # Format classes (Python's primary organizational unit)
        for cls in sorted(module.classes, key=lambda c: c.simple_name):
            lines.extend(self._format_class_python(cls, module))

        # Format standalone functions
        # Collect all method names from classes to filter them out
        method_names = set()
        for cls in module.classes:
            for method in cls.methods:
                method_names.add(method.simple_name)

        if module.functions:
            # Filter out class methods from the functions list
            valid_functions = [
                f
                for f in module.functions
                if self._is_valid_function_name(f.simple_name)
                and f.simple_name not in method_names
            ]
            if valid_functions:
                lines.append("### Functions")
                lines.append("")
                for func in sorted(valid_functions, key=lambda f: f.simple_name):
                    lines.extend(self._format_function_python(func, module))

        # Format types (if any, like TypedDict, Protocol, etc.)
        for typ in sorted(module.types, key=lambda t: t.simple_name):
            lines.extend(self._format_type_python(typ, module))

        return lines

    def _format_class_python(
        self, cls: ClassDefinition, module: ModuleDefinition
    ) -> list[str]:
        """Format a Python class in Pydoc-Markdown style."""
        lines = [f"### {module.module_path}.{cls.simple_name}", ""]

        # Class signature with constructor
        parsed_file = self._find_parsed_file_for_class(module, cls)
        if parsed_file:
            # Generate class signature
            class_sig = f"class {module.module_path}.{cls.simple_name}"

            # Add constructor parameters
            if cls.constructor_params:
                params_str = ", ".join(cls.constructor_params)
                class_sig += f"({params_str})"

            # Add base classes
            if cls.base_classes:
                bases_str = ", ".join(cls.base_classes)
                lines.append("```py")
                lines.append(class_sig)
                lines.append(f"Bases: {bases_str}")
                lines.append("```")
            else:
                lines.append("```py")
                lines.append(class_sig)
                lines.append("```")
            lines.append("")

        # Class documentation
        if cls.docstring:
            lines.append(cls.docstring)
            lines.append("")

        # Methods section
        if cls.methods:
            valid_methods = [
                m for m in cls.methods if self._is_valid_function_name(m.simple_name)
            ]
            if valid_methods:
                lines.append("#### Methods")
                lines.append("")
                for method in sorted(valid_methods, key=lambda m: m.simple_name):
                    lines.extend(self._format_method_python(method, cls, module))

        return lines

    def _format_method_python(
        self,
        method: FunctionDefinition,
        cls: ClassDefinition,
        module: ModuleDefinition,
    ) -> list[str]:
        """Format a Python method in Pydoc-Markdown style."""
        lines = [f"##### {cls.simple_name}.{method.simple_name}", ""]

        # Method signature
        parsed_file = self._find_parsed_file_for_function(module, method)
        if parsed_file:
            signature = self._extract_source(parsed_file, method.node).strip()

            # Remove "def " prefix if present
            signature = signature.removeprefix("def ")

            # Extract function name and parameters
            # Remove the function name and keep only parameters and return type
            if "(" in signature:
                func_name = signature[: signature.index("(")]
                rest = signature[signature.index("(") :]

                # Remove 'self' parameter if present
                if rest.startswith("(self"):
                    # Handle cases: (self), (self, ...), (self,...)
                    if rest.startswith("(self)"):
                        rest = "()" + rest[6:]
                    elif rest.startswith("(self, "):
                        rest = "(" + rest[7:]
                    elif rest.startswith("(self,"):
                        rest = "(" + rest[6:]

                signature = f"{func_name}{rest}"

            lines.append("```py")
            lines.append(f"{cls.simple_name}.{signature}")
            lines.append("```")
            lines.append("")

        # Method documentation
        if method.docstring:
            lines.append(method.docstring)
            lines.append("")

        return lines

    def _format_function_python(
        self, func: FunctionDefinition, module: ModuleDefinition
    ) -> list[str]:
        """Format a standalone Python function."""
        lines = [f"#### {func.simple_name}", ""]

        # Function signature
        parsed_file = self._find_parsed_file_for_function(module, func)
        if parsed_file:
            signature = self._extract_source(parsed_file, func.node)
            lines.append("```py")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        # Documentation
        if func.docstring:
            lines.append(func.docstring)
            lines.append("")

        return lines

    def _format_type_python(
        self, typ: TypeDefinition, module: ModuleDefinition
    ) -> list[str]:
        """Format a Python type (TypedDict, Protocol, etc.)."""
        lines = [f"### {module.module_path}.{typ.simple_name}", ""]

        # Type signature
        parsed_file = self._find_parsed_file_for_type(module, typ)
        if parsed_file:
            signature = self._extract_source(parsed_file, typ.node)
            lines.append("```py")
            lines.append(signature.strip())
            lines.append("```")
            lines.append("")

        # Documentation
        if typ.docstring:
            lines.append(typ.docstring)
            lines.append("")

        # Fields (if any)
        if typ.constructor_params:
            lines.append("**Fields:**")
            lines.append("")
            lines.extend(f"- `{param}`" for param in typ.constructor_params)
            lines.append("")

        return lines

    def _anchor(self, text: str) -> str:
        """Generate markdown anchor from text."""
        anchor = text.lower()
        anchor = anchor.replace("/", "-").replace(".", "-")
        anchor = re.sub(r"[^a-z0-9\-_]", "", anchor)
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
        """Extract just the signature from a Python definition."""
        lines = source.split("\n")
        signature_lines = []

        for line in lines:
            signature_lines.append(line)
            line.strip()

            # Python: colon ends signature (unless inside brackets)
            if ":" in line:
                open_parens = line.count("(") - line.count(")")
                open_brackets = line.count("[") - line.count("]")
                open_braces = line.count("{") - line.count("}")
                if open_parens == 0 and open_brackets == 0 and open_braces == 0:
                    break

        return "\n".join(signature_lines)
