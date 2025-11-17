"""Go API documentation formatter in Godoc style."""

import re

import structlog

from kodit.infrastructure.slicing.code_elements import (
    ClassDefinition,
    FunctionDefinition,
    ModuleDefinition,
    ParsedFile,
    TypeDefinition,
)


class GoAPIDocFormatter:
    """Formats code elements into API documentation markdown."""

    def __init__(self) -> None:
        """Initialize the formatter."""
        self.log = structlog.get_logger(__name__)

    def format_combined_markdown(
        self,
        modules: list[ModuleDefinition],
        language: str,
    ) -> str:
        """Generate Godoc-style markdown for all modules combined."""
        lines = []

        # Generate index of all modules
        lines.append(f"## {language} Index")
        lines.append("")
        lines.extend(
            f"- [{module.module_path}](#{self._anchor(module.module_path)})"
            for module in sorted(modules, key=lambda m: m.module_path)
        )
        lines.append("")

        # Generate documentation for each module
        for module in sorted(modules, key=lambda m: m.module_path):
            lines.extend(self._format_module_section(module))

        return "\n".join(lines)

    def _anchor(self, text: str) -> str:
        """Generate markdown anchor from text."""
        # Convert to lowercase
        anchor = text.lower()

        # Replace slashes and dots with hyphens
        anchor = anchor.replace("/", "-").replace(".", "-")

        # Remove any characters that aren't alphanumeric, hyphens, or underscores
        anchor = re.sub(r"[^a-z0-9\-_]", "", anchor)

        # Replace multiple consecutive hyphens with a single hyphen
        anchor = re.sub(r"-+", "-", anchor)

        # Strip leading/trailing hyphens
        return anchor.strip("-")

    def _format_module_section(self, module: ModuleDefinition) -> list[str]:
        """Generate markdown section for a single module."""
        lines = []

        # Module header and docstring
        lines.append(f"## {module.module_path}")
        lines.append("")
        if module.module_docstring:
            lines.append(module.module_docstring)
            lines.append("")

        # Add subsections in godoc order: constants, types, functions
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
                lines.append("```")
                lines.append(signature.strip())
                lines.append("```")
                lines.append("")
        return lines

    def _format_functions_section(self, module: ModuleDefinition) -> list[str]:
        """Format functions section for a module."""
        if not module.functions:
            return []

        # Filter out invalid function names (minified, anonymous, etc.)
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

        # Format type definitions
        for typ in sorted(module.types, key=lambda t: t.simple_name):
            lines.extend(self._format_type(typ, module))

        # Format class definitions with methods
        for cls in sorted(module.classes, key=lambda c: c.simple_name):
            lines.extend(self._format_class(cls, module))

        return lines

    def _format_source_files_section(self, module: ModuleDefinition) -> list[str]:
        """Format source files section for a module."""
        from pathlib import Path

        # Filter out __init__.py files as they're implementation details
        non_init_files = [
            parsed
            for parsed in module.files
            if Path(parsed.git_file.path).name != "__init__.py"
        ]

        # Only show section if there are non-__init__.py files
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
        # For Go methods, extract receiver type for godoc-style heading
        parsed_file = self._find_parsed_file_for_function(module, func)
        if parsed_file and func.is_method:
            receiver_type = self._extract_go_receiver_type(func.node, parsed_file)
            if receiver_type:
                heading = f"#### func ({receiver_type}) {func.simple_name}"
            else:
                heading = f"#### {func.simple_name}"
        else:
            heading = f"#### {func.simple_name}"

        lines = [heading, ""]

        # Signature
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

    def _format_type(self, typ: TypeDefinition, module: ModuleDefinition) -> list[str]:
        """Format a type in Go-Doc style."""
        lines = [f"#### type {typ.simple_name}", ""]

        # Signature
        parsed_file = self._find_parsed_file_for_type(module, typ)
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

        # Constructor parameters (for structs with fields)
        if typ.constructor_params:
            lines.append("**Fields:**")
            lines.append("")
            lines.extend(f"- `{param}`" for param in typ.constructor_params)
            lines.append("")

        return lines

    def _format_class(
        self, cls: ClassDefinition, module: ModuleDefinition
    ) -> list[str]:
        """Format a class in Go-Doc style."""
        lines = [f"### type {cls.simple_name}", ""]

        # Class signature
        parsed_file = self._find_parsed_file_for_class(module, cls)
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

        # Constructor parameters
        if cls.constructor_params:
            lines.append("**Constructor Parameters:**")
            lines.append("")
            lines.extend(f"- `{param}`" for param in cls.constructor_params)
            lines.append("")

        # Methods - filter out invalid method names
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
        """Format a method in Go-Doc style."""
        lines = [f"#### func ({cls.simple_name}) {method.simple_name}", ""]

        # Method signature
        parsed_file = self._find_parsed_file_for_function(module, method)
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

    def _is_valid_function_name(self, name: str) -> bool:
        """Check if a function name should be included in API documentation."""
        if not name:
            return False

        # Length check - names longer than 255 chars are likely minified code
        if len(name) > 255:
            return False

        # Skip common anonymous/auto-generated function name patterns
        anonymous_patterns = [
            "anonymous",  # Anonymous functions
            "default",  # Default export names in some bundlers
        ]
        if name.lower() in anonymous_patterns:  # noqa: SIM103
            return False

        return True

    def _extract_go_receiver_type(
        self, node: object, parsed_file: ParsedFile
    ) -> str | None:
        """Extract Go receiver type from method declaration."""
        node_type = getattr(node, "type", None)
        if not node_type or node_type != "method_declaration":
            return None

        # Find the parameter_list that represents the receiver
        for child in node.children:  # type: ignore[attr-defined]
            if child.type == "parameter_list":
                # This is the receiver parameter
                for param_child in child.children:
                    if param_child.type == "parameter_declaration":
                        # Extract the type from the parameter
                        return self._extract_go_type_from_param(
                            param_child, parsed_file
                        )
                # If we found the parameter_list but no parameter, break
                break

        return None

    def _extract_go_type_from_param(
        self, param_node: object, parsed_file: ParsedFile
    ) -> str | None:
        """Extract type from Go parameter declaration node."""
        # Look for type children: pointer_type or type_identifier
        for child in param_node.children:  # type: ignore[attr-defined]
            if child.type == "pointer_type" and hasattr(child, "start_byte"):
                # Extract the type being pointed to
                start = child.start_byte
                end = child.end_byte
                type_bytes = parsed_file.source_code[start:end]
                try:
                    return type_bytes.decode("utf-8")
                except UnicodeDecodeError:
                    return None
            if (
                child.type == "type_identifier"
                and hasattr(child, "text")
                and child.text
            ):
                # Direct type identifier
                return child.text.decode("utf-8")

        return None

    def _find_parsed_file_for_function(
        self, module: ModuleDefinition, func: FunctionDefinition
    ) -> ParsedFile | None:
        """Find the parsed file containing a function definition."""
        # Match by file path from FunctionDefinition
        for parsed in module.files:
            if parsed.path == func.file:
                return parsed

        # Fallback: if we can't find by file path, this is an error condition
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
        # Match by file path from TypeDefinition
        for parsed in module.files:
            if parsed.path == typ.file:
                return parsed

        # Fallback: if we can't find by file path, this is an error condition
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
        # Match by file path from ClassDefinition
        for parsed in module.files:
            if parsed.path == cls.file:
                return parsed

        # Fallback: if we can't find by file path, this is an error condition
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
        # First try to match by tree reference
        if hasattr(node, "tree"):
            node_tree = node.tree  # type: ignore[attr-defined]
            for parsed in module.files:
                if parsed.tree == node_tree:
                    return parsed

        # Fallback: if we can't find by tree, this is an error condition
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
            # Extract just the signature
            return self._extract_signature_only(source)
        except (UnicodeDecodeError, IndexError):
            return "<source unavailable>"

    def _extract_signature_only(self, source: str) -> str:
        """Extract just the signature from a definition."""
        lines = source.split("\n")

        # Check if this is a Go type definition
        first_line = lines[0].strip() if lines else ""
        is_go_type = any(keyword in first_line for keyword in [" struct", " interface"])

        if is_go_type:
            # For Go types, include the full definition including the body
            brace_count = 0
            signature_lines = []

            for line in lines:
                signature_lines.append(line)
                # Count braces to find the end of the type definition
                brace_count += line.count("{") - line.count("}")

                # If we've closed all braces, we're done
                if brace_count == 0 and "{" in "".join(signature_lines):
                    break

            return "\n".join(signature_lines)

        # For functions, extract just the signature
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
