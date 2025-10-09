"""AST analyzer for extracting code definitions across multiple languages.

This module provides language-agnostic AST parsing and analysis using tree-sitter.
"""

from collections.abc import Generator
from dataclasses import dataclass
from pathlib import Path

import structlog
from tree_sitter import Node, Parser, Tree
from tree_sitter_language_pack import get_language

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.slicer import LanguageConfig


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
            functions.extend(self._extract_functions(parsed, include_private))
            classes.extend(self._extract_classes(parsed, include_private))
            types.extend(self._extract_types(parsed, include_private))

        return functions, classes, types

    def extract_module_definitions(
        self, parsed_files: list[ParsedFile], *, include_private: bool = False
    ) -> list[ModuleDefinition]:
        """Extract definitions grouped by module."""
        modules = self._group_by_module(parsed_files)

        result = []
        for module_path, module_files in modules.items():
            functions = []
            classes = []
            types = []
            constants = []

            for parsed in module_files:
                functions.extend(self._extract_functions(parsed, include_private))
                classes.extend(self._extract_classes(parsed, include_private))
                types.extend(self._extract_types(parsed, include_private))
                constants.extend(self._extract_constants(parsed, include_private))

            module_doc = self._extract_module_docstring(module_files)

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
        self, parsed: ParsedFile, include_private: bool
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
        self, parsed: ParsedFile, include_private: bool
    ) -> list[ClassDefinition]:
        """Extract class definitions with their methods."""
        return []

    def _extract_types(
        self, parsed: ParsedFile, include_private: bool
    ) -> list[TypeDefinition]:
        """Extract type definitions (enums, interfaces, type aliases, structs)."""
        return []

    def _extract_constants(
        self, parsed: ParsedFile, include_private: bool
    ) -> list[tuple[str, Node]]:
        """Extract public constants."""
        return []

    def _group_by_module(
        self, parsed_files: list[ParsedFile]
    ) -> dict[str, list[ParsedFile]]:
        """Group files by module based on language conventions."""
        modules: dict[str, list[ParsedFile]] = {}
        for parsed in parsed_files:
            module_path = parsed.path.stem
            if module_path not in modules:
                modules[module_path] = []
            modules[module_path].append(parsed)
        return modules

    def _extract_module_docstring(
        self, module_files: list[ParsedFile]
    ) -> str | None:
        """Extract module-level documentation."""
        return None

    def _is_public(self, node: Node, name: str) -> bool:
        """Determine if a definition is public based on language conventions."""
        if self.language == "python":
            return not name.startswith("_")
        if self.language == "go":
            return name[0].isupper() if name else False
        return True

    def _extract_docstring(self, node: Node) -> str | None:
        """Extract documentation comment for a definition."""
        return None

    def _extract_parameters(self, node: Node) -> list[str]:
        """Extract parameter names from a function definition."""
        return []

    def _extract_return_type(self, node: Node) -> str | None:
        """Extract return type from a function definition."""
        return None

    def _is_method(self, node: Node) -> bool:
        """Check if a function is a method (inside a class)."""
        return False
