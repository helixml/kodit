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
        include_private: bool = False,  # noqa: FBT001, FBT002
    ) -> list[APIDocEnrichment]:
        """Extract API documentation enrichments from files.

        Returns a single enrichment per language that combines all modules.

        Args:
            files: List of Git files to extract API docs from
            language: Programming language of the files
            commit_sha: Git commit SHA to use as entity_id
            include_private: Whether to include private functions/classes

        """
        if not files:
            return []

        # Filter out languages that shouldn't have API docs
        if language not in self.SUPPORTED_LANGUAGES:
            self.log.debug("Language not supported for API docs", language=language)
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

        # Filter modules: must have content, not be tests, and have module_path
        modules_with_content = [
            m
            for m in modules
            if self._has_content(m)
            and not self._is_test_module(m)
            and m.module_path  # Exclude modules with empty module_path
        ]

        if not modules_with_content:
            return []

        # Merge modules with the same module_path
        merged_modules = self._merge_modules(modules_with_content)

        # Generate single markdown document for all modules
        markdown_content = self._generate_combined_markdown(
            merged_modules,
            language,
        )

        enrichment = APIDocEnrichment(
            language=language,
            content=markdown_content,
        )

        return [enrichment]

    def _has_content(self, module: ModuleDefinition) -> bool:
        """Check if module has any API elements or documentation."""
        return bool(
            module.functions
            or module.classes
            or module.types
            or module.constants
            or module.module_docstring
        )

    def _is_test_module(self, module: ModuleDefinition) -> bool:
        """Check if a module appears to be a test module.

        Detects test modules based on common patterns:
        - Module path contains 'test', 'tests', or '__tests__' directory
        - Files with '_test' suffix (e.g., foo_test.go)
        - Files with 'test_' prefix (e.g., test_foo.py)
        - Files with '.test.' or '.spec.' in name (e.g., foo.test.js)
        - Files with '_mocks' in name
        """
        from pathlib import Path

        # Check module_path for test directories
        module_path_lower = module.module_path.lower()
        module_path_parts = module_path_lower.split("/")

        # Check if any part of the module path is a test directory
        if any(part in ["test", "tests", "__tests__"] for part in module_path_parts):
            return True

        # Check all files in the module for test file name patterns
        for parsed_file in module.files:
            file_path = Path(parsed_file.git_file.path)
            filename = file_path.name.lower()

            # Check for test file name patterns
            # Use more specific patterns to avoid false positives
            if (
                filename.endswith(("_test.go", "_test.py"))
                or filename.startswith("test_")
                or ".test." in filename
                or ".spec." in filename
                or "_mocks." in filename
                or "_mock." in filename
            ):
                return True

        return False

    def _merge_modules(self, modules: list[ModuleDefinition]) -> list[ModuleDefinition]:
        """Merge modules with the same module_path.

        This is particularly important for Go where multiple files belong to
        the same package/module.
        """
        from collections import defaultdict

        # Group modules by module_path
        modules_by_path: dict[str, list[ModuleDefinition]] = defaultdict(list)
        for module in modules:
            modules_by_path[module.module_path].append(module)

        # Merge modules with same path
        merged: list[ModuleDefinition] = []
        for module_path, module_group in modules_by_path.items():
            if len(module_group) == 1:
                # No merging needed
                merged.append(module_group[0])
            else:
                # Merge all modules in this group
                merged_module = self._merge_module_group(module_path, module_group)
                merged.append(merged_module)

        return merged

    def _merge_module_group(
        self, module_path: str, module_group: list[ModuleDefinition]
    ) -> ModuleDefinition:
        """Merge a group of modules with the same path into a single module."""
        # Collect all files
        all_files = []
        for mod in module_group:
            all_files.extend(mod.files)

        # Collect all functions
        all_functions = []
        for mod in module_group:
            all_functions.extend(mod.functions)

        # Collect all classes
        all_classes = []
        for mod in module_group:
            all_classes.extend(mod.classes)

        # Collect all types
        all_types = []
        for mod in module_group:
            all_types.extend(mod.types)

        # Collect all constants
        all_constants = []
        for mod in module_group:
            all_constants.extend(mod.constants)

        # Find first non-empty docstring
        module_docstring = ""
        for mod in module_group:
            if mod.module_docstring:
                module_docstring = mod.module_docstring
                break

        # Create merged module
        return ModuleDefinition(
            module_path=module_path,
            module_docstring=module_docstring,
            files=all_files,
            functions=all_functions,
            classes=all_classes,
            types=all_types,
            constants=all_constants,
        )

    def _is_valid_function_name(self, name: str) -> bool:
        """Check if a function name should be included in API documentation.

        Filters out:
        - Names longer than 255 characters (likely minified code)
        - Anonymous or auto-generated function names
        - Short minified names (2-3 chars with digits)
        """
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

    def _generate_combined_markdown(
        self,
        modules: list[ModuleDefinition],
        language: str,
    ) -> str:
        """Generate Godoc-style markdown for all modules combined.

        Organizes content by module path, with types and functions grouped
        within each module section.
        """
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
            lines.extend(self._generate_module_section(module))

        return "\n".join(lines)

    def _anchor(self, text: str) -> str:
        """Generate markdown anchor from text.

        Follows GitHub-flavored markdown heading ID generation:
        - Convert to lowercase
        - Replace spaces with hyphens
        - Remove punctuation except hyphens and underscores
        - Replace slashes and dots with hyphens
        """
        import re

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

    def _generate_module_section(self, module: ModuleDefinition) -> list[str]:
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

        lines = ["### Source Files", ""]
        # Filter out __init__.py files as they're implementation details
        # The module itself represents the package
        non_init_files = [
            parsed
            for parsed in module.files
            if Path(parsed.git_file.path).name != "__init__.py"
        ]
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
        parsed_file = self._find_parsed_file_for_function(module, func)
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

    def _extract_go_receiver_type(
        self, node: object, parsed_file: ParsedFile
    ) -> str | None:
        """Extract Go receiver type from method declaration.

        Returns the receiver type in godoc format.
        Strips the parameter name, keeping only the type.
        """
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
        # Log a warning and return None to make the error visible
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
        # Log a warning and return None to make the error visible
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
        # Log a warning and return None to make the error visible
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
        # Log a warning and return None to make the error visible
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
        """Extract just the signature from a definition.

        This removes function bodies and only keeps the declaration/signature.
        For Go types (structs, interfaces), includes the full definition.
        """
        lines = source.split("\n")

        # Check if this is a Go type definition
        # (starts with type name followed by struct/interface)
        first_line = lines[0].strip() if lines else ""
        is_go_type = any(keyword in first_line for keyword in [" struct", " interface"])

        if is_go_type:
            # For Go types, include the full definition including the body
            # Find the matching closing brace
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
