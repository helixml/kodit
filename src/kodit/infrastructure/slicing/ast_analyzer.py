"""AST analyzer for extracting code definitions across multiple languages.

This module provides language-agnostic AST parsing and analysis using tree-sitter.
"""

from collections.abc import Generator
from dataclasses import dataclass
from pathlib import Path
from typing import Any, ClassVar

import structlog
from tree_sitter import Node, Parser, Tree
from tree_sitter_language_pack import get_language

from kodit.domain.entities.git import GitFile


class LanguageConfig:
    """Language-specific configuration."""

    CONFIGS: ClassVar[dict[str, dict[str, Any]]] = {
        "python": {
            "function_nodes": ["function_definition"],
            "method_nodes": [],
            "call_node": "call",
            "import_nodes": ["import_statement", "import_from_statement"],
            "extension": ".py",
            "name_field": None,  # Use identifier child
        },
        "java": {
            "function_nodes": ["method_declaration"],
            "method_nodes": [],
            "call_node": "method_invocation",
            "import_nodes": ["import_declaration"],
            "extension": ".java",
            "name_field": None,
        },
        "c": {
            "function_nodes": ["function_definition"],
            "method_nodes": [],
            "call_node": "call_expression",
            "import_nodes": ["preproc_include"],
            "extension": ".c",
            "name_field": "declarator",
        },
        "cpp": {
            "function_nodes": ["function_definition"],
            "method_nodes": [],
            "call_node": "call_expression",
            "import_nodes": ["preproc_include", "using_declaration"],
            "extension": ".cpp",
            "name_field": "declarator",
        },
        "rust": {
            "function_nodes": ["function_item"],
            "method_nodes": [],
            "call_node": "call_expression",
            "import_nodes": ["use_declaration", "extern_crate_declaration"],
            "extension": ".rs",
            "name_field": "name",
        },
        "go": {
            "function_nodes": ["function_declaration"],
            "method_nodes": ["method_declaration"],
            "call_node": "call_expression",
            "import_nodes": ["import_declaration"],
            "extension": ".go",
            "name_field": None,
        },
        "javascript": {
            "function_nodes": [
                "function_declaration",
                "function_expression",
                "arrow_function",
            ],
            "method_nodes": [],
            "call_node": "call_expression",
            "import_nodes": ["import_statement", "import_declaration"],
            "extension": ".js",
            "name_field": None,
        },
        "csharp": {
            "function_nodes": ["method_declaration"],
            "method_nodes": ["constructor_declaration"],
            "call_node": "invocation_expression",
            "import_nodes": ["using_directive"],
            "extension": ".cs",
            "name_field": None,
        },
        "html": {
            "function_nodes": ["script_element", "style_element"],
            "method_nodes": ["element"],  # Elements with id/class attributes
            "call_node": "attribute",
            "import_nodes": ["script_element", "element"],  # script and link elements
            "extension": ".html",
            "name_field": None,
        },
        "css": {
            "function_nodes": ["rule_set", "keyframes_statement"],
            "method_nodes": ["media_statement"],
            "call_node": "call_expression",
            "import_nodes": ["import_statement"],
            "extension": ".css",
            "name_field": None,
        },
    }

    # Aliases
    CONFIGS["c++"] = CONFIGS["cpp"]
    CONFIGS["typescript"] = CONFIGS["javascript"]
    CONFIGS["ts"] = CONFIGS["javascript"]
    CONFIGS["js"] = CONFIGS["javascript"]
    CONFIGS["c#"] = CONFIGS["csharp"]
    CONFIGS["cs"] = CONFIGS["csharp"]


@dataclass
class ParsedFile:
    """Result of parsing a single file with tree-sitter."""

    path: Path
    git_file: GitFile
    tree: Tree
    source_code: bytes


@dataclass
class FunctionDefinition:
    """Information about a function or method definition."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
    simple_name: str
    is_public: bool
    is_method: bool
    docstring: str | None
    parameters: list[str]
    return_type: str | None


@dataclass
class ClassDefinition:
    """Information about a class definition."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
    simple_name: str
    is_public: bool
    docstring: str | None
    methods: list[FunctionDefinition]
    base_classes: list[str]


@dataclass
class TypeDefinition:
    """Information about a type definition (enum, interface, type alias)."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str
    simple_name: str
    is_public: bool
    docstring: str | None
    kind: str


@dataclass
class ModuleDefinition:
    """All definitions in a module, grouped by language conventions."""

    module_path: str
    files: list[ParsedFile]
    functions: list[FunctionDefinition]
    classes: list[ClassDefinition]
    types: list[TypeDefinition]
    constants: list[tuple[str, Node]]
    module_docstring: str | None


class ASTAnalyzer:
    """Language-agnostic AST analyzer.

    Parses files with tree-sitter and extracts structured information about
    definitions (functions, classes, types). Used by both Slicer (for code
    snippets) and other consumers (e.g., API documentation extraction, module
    hierarchy analysis).
    """

    def __init__(self, language: str) -> None:
        """Initialize analyzer for a specific language."""
        self.language = language.lower()
        config = LanguageConfig.CONFIGS.get(self.language)
        if not config:
            raise ValueError(f"Unsupported language: {language}")
        self.config = config

        ts_language = get_language(self._get_tree_sitter_name())  # type: ignore[arg-type]
        self.parser = Parser(ts_language)
        self.log = structlog.get_logger(__name__)

    def parse_files(self, files: list[GitFile]) -> list[ParsedFile]:
        """Parse files into AST trees."""
        parsed = []
        for git_file in files:
            path = Path(git_file.path)
            if not path.exists():
                self.log.debug("Skipping non-existent file", path=str(path))
                continue

            try:
                with path.open("rb") as f:
                    source_code = f.read()

                tree = self.parser.parse(source_code)
                parsed.append(
                    ParsedFile(
                        path=path,
                        git_file=git_file,
                        tree=tree,
                        source_code=source_code,
                    )
                )
            except OSError as e:
                self.log.warning("Failed to parse file", path=str(path), error=str(e))
                continue

        return parsed

    def extract_definitions(
        self,
        parsed_files: list[ParsedFile],
        *,
        include_private: bool = True,
    ) -> tuple[list[FunctionDefinition], list[ClassDefinition], list[TypeDefinition]]:
        """Extract all definitions from parsed files."""
        functions = []
        classes = []
        types = []

        for parsed in parsed_files:
            functions.extend(
                self._extract_functions(parsed, include_private=include_private)
            )
            classes.extend(
                self._extract_classes(parsed, include_private=include_private)
            )
            types.extend(
                self._extract_types(parsed, include_private=include_private)
            )

        return functions, classes, types

    def extract_module_definitions(
        self, parsed_files: list[ParsedFile], *, include_private: bool = False
    ) -> list[ModuleDefinition]:
        """Extract definitions grouped by module."""
        modules = self._group_by_module(parsed_files)

        result = []
        for module_files in modules.values():
            functions = []
            classes = []
            types = []
            constants = []

            for parsed in module_files:
                functions.extend(
                    self._extract_functions(parsed, include_private=include_private)
                )
                classes.extend(
                    self._extract_classes(parsed, include_private=include_private)
                )
                types.extend(
                    self._extract_types(parsed, include_private=include_private)
                )
                constants.extend(
                    self._extract_constants(parsed, include_private=include_private)
                )

            module_doc = self._extract_module_docstring(module_files)

            # Extract the actual module path from the file using Tree-sitter
            module_path = self._extract_module_path(module_files[0])

            result.append(
                ModuleDefinition(
                    module_path=module_path,
                    files=module_files,
                    functions=functions,
                    classes=classes,
                    types=types,
                    constants=constants,
                    module_docstring=module_doc,
                )
            )

        return result

    def _get_tree_sitter_name(self) -> str:
        """Map language name to tree-sitter language name."""
        mapping = {
            "c++": "cpp",
            "c#": "csharp",
            "cs": "csharp",
            "js": "javascript",
            "ts": "typescript",
        }
        return mapping.get(self.language, self.language)

    def _walk_tree(self, node: Node) -> Generator[Node, None, None]:
        """Walk the AST tree, yielding all nodes."""
        queue = [node]
        visited: set[int] = set()

        while queue:
            current = queue.pop(0)
            node_id = id(current)
            if node_id in visited:
                continue
            visited.add(node_id)

            yield current
            queue.extend(current.children)

    def _is_function_definition(self, node: Node) -> bool:
        """Check if node is a function definition."""
        return node.type in (
            self.config["function_nodes"] + self.config["method_nodes"]
        )

    def _extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a function definition node."""
        if self.language == "html":
            return self._extract_html_element_name(node)
        if self.language == "css":
            return self._extract_css_rule_name(node)
        if self.language == "go" and node.type == "method_declaration":
            return self._extract_go_method_name(node)
        if self.language in ["c", "cpp"] and self.config["name_field"]:
            return self._extract_c_cpp_function_name(node)
        if self.language == "rust" and self.config["name_field"]:
            return self._extract_rust_function_name(node)
        return self._extract_default_function_name(node)

    def _extract_go_method_name(self, node: Node) -> str | None:
        """Extract method name from Go method declaration."""
        for child in node.children:
            if child.type == "field_identifier" and child.text is not None:
                return child.text.decode("utf-8")
        return None

    def _extract_c_cpp_function_name(self, node: Node) -> str | None:
        """Extract function name from C/C++ function definition."""
        declarator = node.child_by_field_name(self.config["name_field"])
        if not declarator:
            return None

        if declarator.type == "function_declarator":
            for child in declarator.children:
                if child.type == "identifier" and child.text is not None:
                    return child.text.decode("utf-8")
        elif declarator.type == "identifier" and declarator.text is not None:
            return declarator.text.decode("utf-8")
        return None

    def _extract_rust_function_name(self, node: Node) -> str | None:
        """Extract function name from Rust function definition."""
        name_node = node.child_by_field_name(self.config["name_field"])
        if name_node and name_node.type == "identifier" and name_node.text is not None:
            return name_node.text.decode("utf-8")
        return None

    def _extract_html_element_name(self, node: Node) -> str | None:
        """Extract meaningful name from HTML element."""
        if node.type == "script_element":
            return "script"
        if node.type == "style_element":
            return "style"
        if node.type == "element":
            return self._extract_html_element_info(node)
        return None

    def _extract_html_element_info(self, node: Node) -> str | None:
        """Extract element info with ID or class."""
        for child in node.children:
            if child.type == "start_tag":
                tag_name = self._get_tag_name(child)
                element_id = self._get_element_id(child)
                class_name = self._get_element_class(child)

                if element_id:
                    return f"{tag_name or 'element'}#{element_id}"
                if class_name:
                    return f"{tag_name or 'element'}.{class_name}"
                if tag_name:
                    return tag_name
        return None

    def _get_tag_name(self, start_tag: Node) -> str | None:
        """Get tag name from start_tag node."""
        for child in start_tag.children:
            if child.type == "tag_name" and child.text:
                try:
                    return child.text.decode("utf-8")
                except UnicodeDecodeError:
                    return None
        return None

    def _get_element_id(self, start_tag: Node) -> str | None:
        """Get element ID from start_tag node."""
        return self._get_attribute_value(start_tag, "id")

    def _get_element_class(self, start_tag: Node) -> str | None:
        """Get first class name from start_tag node."""
        class_value = self._get_attribute_value(start_tag, "class")
        return class_value.split()[0] if class_value else None

    def _get_attribute_value(self, start_tag: Node, attr_name: str) -> str | None:
        """Get attribute value from start_tag node."""
        for child in start_tag.children:
            if child.type == "attribute":
                name = self._get_attr_name(child)
                if name == attr_name:
                    return self._get_attr_value(child)
        return None

    def _get_attr_name(self, attr_node: Node) -> str | None:
        """Get attribute name."""
        for child in attr_node.children:
            if child.type == "attribute_name" and child.text:
                try:
                    return child.text.decode("utf-8")
                except UnicodeDecodeError:
                    return None
        return None

    def _get_attr_value(self, attr_node: Node) -> str | None:
        """Get attribute value."""
        for child in attr_node.children:
            if child.type == "quoted_attribute_value":
                for val_child in child.children:
                    if val_child.type == "attribute_value" and val_child.text:
                        try:
                            return val_child.text.decode("utf-8")
                        except UnicodeDecodeError:
                            return None
        return None

    def _extract_css_rule_name(self, node: Node) -> str | None:
        """Extract meaningful name from CSS rule."""
        if node.type == "rule_set":
            return self._extract_css_selector(node)
        if node.type == "keyframes_statement":
            return self._extract_keyframes_name(node)
        if node.type == "media_statement":
            return "@media"
        return None

    def _extract_css_selector(self, rule_node: Node) -> str | None:
        """Extract CSS selector from rule_set."""
        for child in rule_node.children:
            if child.type == "selectors":
                selector_parts = []
                for selector_child in child.children:
                    part = self._get_selector_part(selector_child)
                    if part:
                        selector_parts.append(part)
                if selector_parts:
                    return "".join(selector_parts[:2])
        return None

    def _get_selector_part(self, selector_node: Node) -> str | None:
        """Get a single selector part."""
        if selector_node.type == "class_selector":
            return self._extract_class_selector(selector_node)
        if selector_node.type == "id_selector":
            return self._extract_id_selector(selector_node)
        if selector_node.type == "type_selector" and selector_node.text:
            return selector_node.text.decode("utf-8")
        return None

    def _extract_class_selector(self, node: Node) -> str | None:
        """Extract class selector name."""
        for child in node.children:
            if child.type == "class_name":
                for name_child in child.children:
                    if name_child.type == "identifier" and name_child.text:
                        return f".{name_child.text.decode('utf-8')}"
        return None

    def _extract_id_selector(self, node: Node) -> str | None:
        """Extract ID selector name."""
        for child in node.children:
            if child.type == "id_name":
                for name_child in child.children:
                    if name_child.type == "identifier" and name_child.text:
                        return f"#{name_child.text.decode('utf-8')}"
        return None

    def _extract_keyframes_name(self, node: Node) -> str | None:
        """Extract keyframes animation name."""
        for child in node.children:
            if child.type == "keyframes_name" and child.text:
                return f"@keyframes-{child.text.decode('utf-8')}"
        return None

    def _extract_default_function_name(self, node: Node) -> str | None:
        """Extract function name using default identifier search."""
        for child in node.children:
            if child.type == "identifier" and child.text is not None:
                return child.text.decode("utf-8")
        return None

    def _qualify_name(self, node: Node, file_path: Path) -> str | None:
        """Create qualified name for a function node."""
        function_name = self._extract_function_name(node)
        if not function_name:
            return None

        module_name = file_path.stem
        return f"{module_name}.{function_name}"

    def _extract_functions(
        self, parsed: ParsedFile, *, include_private: bool
    ) -> list[FunctionDefinition]:
        """Extract function definitions from a parsed file."""
        functions = []

        for node in self._walk_tree(parsed.tree.root_node):
            if self._is_function_definition(node):
                qualified_name = self._qualify_name(node, parsed.path)
                if not qualified_name:
                    continue

                simple_name = self._extract_function_name(node)
                if not simple_name:
                    continue

                is_public = self._is_public(node, simple_name)
                if not include_private and not is_public:
                    continue

                span = (node.start_byte, node.end_byte)
                docstring = self._extract_docstring(node)
                parameters = self._extract_parameters(node)
                return_type = self._extract_return_type(node)
                is_method = self._is_method(node)

                functions.append(
                    FunctionDefinition(
                        file=parsed.path,
                        node=node,
                        span=span,
                        qualified_name=qualified_name,
                        simple_name=simple_name,
                        is_public=is_public,
                        is_method=is_method,
                        docstring=docstring,
                        parameters=parameters,
                        return_type=return_type,
                    )
                )

        return functions

    def _extract_classes(
        self, parsed: ParsedFile, *, include_private: bool
    ) -> list[ClassDefinition]:
        """Extract class definitions with their methods."""
        if self.language == "python":
            return self._extract_python_classes(parsed, include_private=include_private)
        # For other languages, not yet implemented
        return []

    def _extract_python_classes(
        self, parsed: ParsedFile, *, include_private: bool
    ) -> list[ClassDefinition]:
        """Extract Python class definitions."""
        classes = []

        for node in self._walk_tree(parsed.tree.root_node):
            if node.type == "class_definition":
                # Extract class name
                class_name = None
                for child in node.children:
                    if child.type == "identifier" and child.text:
                        class_name = child.text.decode("utf-8")
                        break

                if not class_name:
                    continue

                # Check if public
                is_public = self._is_public(node, class_name)
                if not include_private and not is_public:
                    continue

                # Extract docstring
                docstring = self._extract_docstring(node)

                # Extract methods (functions defined inside the class)
                methods = self._extract_class_methods(node, parsed, include_private)

                # Extract base classes
                base_classes = self._extract_base_classes(node)

                qualified_name = f"{parsed.path.stem}.{class_name}"
                span = (node.start_byte, node.end_byte)

                classes.append(
                    ClassDefinition(
                        file=parsed.path,
                        node=node,
                        span=span,
                        qualified_name=qualified_name,
                        simple_name=class_name,
                        is_public=is_public,
                        docstring=docstring,
                        methods=methods,
                        base_classes=base_classes,
                    )
                )

        return classes

    def _extract_class_methods(  # noqa: C901
        self, class_node: Node, parsed: ParsedFile, include_private: bool  # noqa: FBT001
    ) -> list[FunctionDefinition]:
        """Extract methods from a class definition."""
        methods = []

        # Find the block (class body)
        for child in class_node.children:
            if child.type == "block":
                # Look for function_definition nodes in the block
                for block_child in child.children:
                    if block_child.type == "function_definition":
                        method_name = None
                        for func_child in block_child.children:
                            if func_child.type == "identifier" and func_child.text:
                                method_name = func_child.text.decode("utf-8")
                                break

                        if not method_name:
                            continue

                        # Check if public
                        is_public = self._is_public(block_child, method_name)
                        if not include_private and not is_public:
                            continue

                        # Extract docstring
                        docstring = self._extract_docstring(block_child)

                        # Get class name for qualified name
                        class_name = None
                        for class_child in class_node.children:
                            if class_child.type == "identifier" and class_child.text:
                                class_name = class_child.text.decode("utf-8")
                                break

                        qualified_name = (
                            f"{parsed.path.stem}.{class_name}.{method_name}"
                        )
                        span = (block_child.start_byte, block_child.end_byte)

                        methods.append(
                            FunctionDefinition(
                                file=parsed.path,
                                node=block_child,
                                span=span,
                                qualified_name=qualified_name,
                                simple_name=method_name,
                                is_public=is_public,
                                is_method=True,
                                docstring=docstring,
                                parameters=[],
                                return_type=None,
                            )
                        )

        return methods

    def _extract_base_classes(self, class_node: Node) -> list[str]:
        """Extract base class names from a class definition."""
        base_classes: list[str] = []

        # Look for argument_list (the inheritance list in Python)
        for child in class_node.children:
            if child.type == "argument_list":
                base_classes.extend(
                    arg_child.text.decode("utf-8")
                    for arg_child in child.children
                    if arg_child.type == "identifier" and arg_child.text
                )

        return base_classes

    def _extract_types(
        self, parsed: ParsedFile, *, include_private: bool
    ) -> list[TypeDefinition]:
        """Extract type definitions (enums, interfaces, type aliases, structs)."""
        if self.language == "go":
            return self._extract_go_types(parsed, include_private=include_private)
        # For other languages, not yet implemented
        return []

    def _extract_go_types(
        self, parsed: ParsedFile, *, include_private: bool
    ) -> list[TypeDefinition]:
        """Extract Go type definitions."""
        types = []

        for node in self._walk_tree(parsed.tree.root_node):
            if node.type == "type_declaration":
                # Extract type_spec child which contains the actual type info
                for child in node.children:
                    if child.type == "type_spec":
                        type_def = self._extract_go_type_from_spec(
                            child, parsed, include_private=include_private
                        )
                        if type_def:
                            types.append(type_def)

        return types

    def _extract_go_type_from_spec(
        self, type_spec_node: Node, parsed: ParsedFile, *, include_private: bool
    ) -> TypeDefinition | None:
        """Extract a single Go type definition from a type_spec node."""
        # Get type name (type_identifier)
        type_name = None
        type_kind = "type"

        for child in type_spec_node.children:
            if child.type == "type_identifier" and child.text:
                type_name = child.text.decode("utf-8")
            elif child.type == "struct_type":
                type_kind = "struct"
            elif child.type == "interface_type":
                type_kind = "interface"
            elif child.type in ["slice_type", "array_type", "pointer_type"]:
                type_kind = "alias"
            elif child.type == "map_type":
                type_kind = "map"

        if not type_name:
            return None

        # Check if public
        is_public = self._is_public(type_spec_node, type_name)
        if not include_private and not is_public:
            return None

        # Extract docstring (comment before the type declaration)
        parent_node = type_spec_node.parent
        if not parent_node:
            return None

        docstring = self._extract_go_type_comment(parent_node)

        qualified_name = f"{parsed.path.stem}.{type_name}"
        # Use the parent type_declaration node to include the "type" keyword
        span = (parent_node.start_byte, parent_node.end_byte)

        return TypeDefinition(
            file=parsed.path,
            node=parent_node,  # Use parent to include "type" keyword
            span=span,
            qualified_name=qualified_name,
            simple_name=type_name,
            is_public=is_public,
            docstring=docstring,
            kind=type_kind,
        )

    def _extract_go_type_comment(self, type_decl_node: Node) -> str | None:
        """Extract comment before a Go type declaration."""
        # Look for comment node immediately before the type_declaration
        parent = type_decl_node.parent
        if not parent:
            return None

        # Find the index of the type_declaration in parent's children
        type_decl_index = None
        for i, child in enumerate(parent.children):
            if child == type_decl_node:
                type_decl_index = i
                break

        if type_decl_index is None or type_decl_index == 0:
            return None

        # Check the previous sibling
        prev_sibling = parent.children[type_decl_index - 1]
        if prev_sibling.type == "comment" and prev_sibling.text:
            comment_text = prev_sibling.text.decode("utf-8")
            # Remove leading // and whitespace
            return comment_text.lstrip("/").strip()

        return None

    def _extract_constants(
        self, parsed: ParsedFile, *, include_private: bool
    ) -> list[tuple[str, Node]]:
        """Extract public constants."""
        _ = parsed, include_private  # Mark as intentionally unused for now
        return []

    def _group_by_module(
        self, parsed_files: list[ParsedFile]
    ) -> dict[str, list[ParsedFile]]:
        """Create one module per file.

        Each file becomes its own module with a unique key.
        The module_path is extracted separately for display purposes.
        """
        modules: dict[str, list[ParsedFile]] = {}
        for idx, parsed in enumerate(parsed_files):
            # Use file path + index as unique key to prevent collisions
            # The actual module_path for display is extracted later
            unique_key = f"{parsed.path}#{idx}"
            modules[unique_key] = [parsed]
        return modules

    def _extract_module_path(self, parsed: ParsedFile) -> str:
        """Extract module/package path based on language conventions.

        Uses Tree-sitter to parse package declarations from source code.
        For languages without explicit package declarations (like Python),
        uses the file path structure to build a fully qualified module path.
        """
        if self.language == "go":
            return self._extract_go_package_path(parsed)
        if self.language == "java":
            return self._extract_java_package_name(parsed)
        if self.language == "python":
            return self._extract_python_module_path(parsed)
        # Default: use file path without extension
        return self._extract_path_based_module(parsed)

    def _extract_go_package_name(self, parsed: ParsedFile) -> str:
        """Extract Go package name (last component) from package declaration."""
        root = parsed.tree.root_node
        for child in root.children:
            if child.type == "package_clause":
                for package_child in child.children:
                    if (
                        package_child.type == "package_identifier"
                        and package_child.text
                    ):
                        return package_child.text.decode("utf-8")
        # Fallback to file stem
        return parsed.path.stem

    def _extract_go_package_path(self, parsed: ParsedFile) -> str:
        """Extract full Go package path using directory structure.

        Go packages are identified by their import path, which is
        typically the directory path. The package name is the last component.
        """
        # Get package name from source
        package_name = self._extract_go_package_name(parsed)

        # Build path from directory structure
        file_path = Path(parsed.git_file.path)
        clean_path = self._clean_path_for_module(file_path)
        dir_path = clean_path.parent

        # Convert to Go-style import path (use / separator)
        if str(dir_path) != ".":
            dir_str = str(dir_path).replace("\\", "/")
            # Check if package name is already the last component of the path
            # to avoid duplication like "agent/agent"
            if dir_str.endswith("/" + package_name) or dir_str == package_name:
                return dir_str
            return f"{dir_str}/{package_name}"
        return package_name

    def _extract_python_module_path(self, parsed: ParsedFile) -> str:
        """Extract Python module path from file path structure.

        Python modules are identified by their file path, with / replaced by dots.
        Attempts to extract a clean relative path by removing common prefixes.
        __init__.py files represent the parent directory as a module.
        """
        file_path = Path(parsed.git_file.path)

        # Try to make it relative and clean
        clean_path = self._clean_path_for_module(file_path)

        # For __init__.py, the module is just the directory name
        if clean_path.name == "__init__.py":
            # Just directory parts, no filename
            module_parts = list(clean_path.parts[:-1])
        else:
            # For regular files, include filename
            module_parts = list(clean_path.parts[:-1])  # Get directory parts
            module_parts.append(clean_path.stem)  # Add filename without extension

        # Filter out empty parts and convert to dotted notation
        module_path = ".".join(p for p in module_parts if p and p != ".")

        # For top-level __init__.py where clean path has no parent directory,
        # use the actual parent directory name from the full path
        if not module_path and clean_path.name == "__init__.py":
            # Get the parent directory from the original file path
            parent_dir = file_path.parent.name
            if parent_dir and parent_dir != ".":
                return parent_dir
            return ""

        return module_path if module_path else clean_path.stem

    def _extract_path_based_module(self, parsed: ParsedFile) -> str:
        """Extract module path based on file path for languages without declarations."""
        file_path = Path(parsed.git_file.path)

        # Try to make it relative and clean
        clean_path = self._clean_path_for_module(file_path)

        # Remove extension and convert to module path
        module_parts = list(clean_path.parts[:-1])  # Get directory parts
        module_parts.append(clean_path.stem)  # Add filename without extension

        # Filter out empty parts and convert to dotted notation
        module_path = ".".join(p for p in module_parts if p and p != ".")
        return module_path if module_path else clean_path.stem

    def _clean_path_for_module(self, file_path: Path) -> Path:
        """Clean a file path to extract a reasonable module path.

        Attempts to remove common repository root indicators like 'src',
        'lib', project directories, etc. to get a clean module path that
        represents the full import path a user would use.
        """
        parts = list(file_path.parts)

        # If it's already relative, just return it
        if not file_path.is_absolute():
            return file_path

        # Special case: if this is test data (contains /data/<language>/),
        # return everything after the language directory
        test_languages = {
            "go",
            "python",
            "java",
            "javascript",
            "typescript",
            "c",
            "cpp",
            "rust",
            "csharp",
        }
        for i in range(len(parts) - 1):
            if (
                parts[i] == "data"
                and i + 1 < len(parts)
                and parts[i + 1] in test_languages
                and i + 2 < len(parts)
            ):
                # Return everything after the language directory
                return Path(*parts[i + 2 :])

        # Try to find common source root markers and return everything after them
        common_roots = {"src", "lib", "pkg", "internal", "app"}
        for i, part in enumerate(parts):
            if part in common_roots:
                # Return path from this point forward (after the root marker)
                if i + 1 < len(parts):
                    return Path(*parts[i + 1 :])
                return file_path

        # If no common root found, look for go.mod, package.json, pyproject.toml
        # and return the path relative to that directory
        # For now, return everything after the last "src-like" directory
        # or just the filename if nothing found
        if len(parts) >= 2:
            return Path(*parts[-2:])

        return file_path

    def _extract_java_package_name(self, parsed: ParsedFile) -> str:
        """Extract Java package name from package declaration."""
        root = parsed.tree.root_node
        for child in root.children:
            if child.type == "package_declaration":
                for package_child in child.children:
                    if package_child.type == "scoped_identifier" and package_child.text:
                        return package_child.text.decode("utf-8")
                    if package_child.type == "identifier" and package_child.text:
                        return package_child.text.decode("utf-8")
        # Fallback to file stem
        return parsed.path.stem

    def _extract_module_docstring(
        self, module_files: list[ParsedFile]
    ) -> str | None:
        """Extract module-level documentation."""
        if self.language == "python":
            # For Python, extract docstring from __init__.py or first file
            for parsed in module_files:
                if parsed.path.name == "__init__.py":
                    # Extract module docstring from __init__.py
                    return self._extract_python_docstring(parsed.tree.root_node)
            # If no __init__.py, try first file
            if module_files:
                return self._extract_python_docstring(module_files[0].tree.root_node)
        return None

    def _is_public(self, node: Node, name: str) -> bool:
        """Determine if a definition is public based on language conventions."""
        _ = node  # Mark as intentionally unused for now
        if self.language == "python":
            return not name.startswith("_")
        if self.language == "go":
            return name[0].isupper() if name else False
        return True

    def _extract_docstring(self, node: Node) -> str | None:
        """Extract documentation comment for a definition."""
        if self.language == "go":
            return self._extract_go_function_comment(node)
        if self.language == "python":
            return self._extract_python_docstring(node)
        # For other languages, not yet implemented
        return None

    def _extract_python_docstring(self, node: Node) -> str | None:  # noqa: C901
        """Extract Python docstring from function, class, or module.

        Python docstrings are string literals that appear as the first statement
        in a function, class, or module body.
        """
        # Look for a block (function body, class body, or module)
        body_node = None

        if node.type in {"function_definition", "class_definition"}:
            # Find the block child
            for child in node.children:
                if child.type == "block":
                    body_node = child
                    break
        elif node.type == "module":
            # Module node is already the body
            body_node = node

        if not body_node:
            return None

        # Look for the first expression_statement containing a string
        for child in body_node.children:
            if child.type == "expression_statement":
                # Check if it contains a string node
                for expr_child in child.children:
                    if expr_child.type == "string" and expr_child.text:
                        # Extract and clean the docstring
                        docstring_bytes = expr_child.text
                        try:
                            docstring_text = docstring_bytes.decode("utf-8")
                            # Remove triple quotes and extra whitespace
                            docstring_text = docstring_text.strip()
                            # Remove leading/trailing quotes
                            for quote in ['"""', "'''", '"', "'"]:
                                starts = docstring_text.startswith(quote)
                                ends = docstring_text.endswith(quote)
                                if starts and ends:
                                    quote_len = len(quote)
                                    docstring_text = docstring_text[
                                        quote_len:-quote_len
                                    ]
                                    break
                            return docstring_text.strip()
                        except UnicodeDecodeError:
                            return None
                # Found expression_statement but no string - stop looking
                break

        return None

    def _extract_go_function_comment(self, func_node: Node) -> str | None:
        """Extract comment before a Go function or method declaration."""
        parent = func_node.parent
        if not parent:
            return None

        # Find the index of the function in parent's children
        func_index = None
        for i, child in enumerate(parent.children):
            if child == func_node:
                func_index = i
                break

        if func_index is None or func_index == 0:
            return None

        # Check the previous sibling
        prev_sibling = parent.children[func_index - 1]
        if prev_sibling.type == "comment" and prev_sibling.text:
            comment_text = prev_sibling.text.decode("utf-8")
            # Remove leading // and whitespace
            return comment_text.lstrip("/").strip()

        return None

    def _extract_parameters(self, node: Node) -> list[str]:
        """Extract parameter names from a function definition."""
        _ = node  # Mark as intentionally unused for now
        return []

    def _extract_return_type(self, node: Node) -> str | None:
        """Extract return type from a function definition."""
        _ = node  # Mark as intentionally unused for now
        return None

    def _is_method(self, node: Node) -> bool:
        """Check if a function is a method (inside a class)."""
        # For Go, check if it's a method_declaration node type
        if self.language == "go":
            return node.type == "method_declaration"
        # For other languages, could check if parent is a class node
        return False
