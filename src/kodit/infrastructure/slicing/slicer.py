"""Complete self-contained analyzer for kodit-slicer.

This module combines all necessary functionality without external dependencies
on the legacy domain/application/infrastructure layers.
"""

from collections import defaultdict
from collections.abc import Generator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, ClassVar

import structlog
from tree_sitter import Node, Parser, Tree
from tree_sitter_language_pack import get_language

from kodit.domain.entities import File, Snippet
from kodit.domain.value_objects import LanguageMapping


@dataclass
class FunctionInfo:
    """Information about a function definition."""

    file: Path
    node: Node
    span: tuple[int, int]
    qualified_name: str


@dataclass
class AnalyzerState:
    """Central state for the dependency analysis."""

    parser: Parser
    files: list[Path] = field(default_factory=list)
    asts: dict[Path, Tree] = field(default_factory=dict)
    def_index: dict[str, FunctionInfo] = field(default_factory=dict)
    call_graph: dict[str, set[str]] = field(default_factory=lambda: defaultdict(set))
    reverse_calls: dict[str, set[str]] = field(default_factory=lambda: defaultdict(set))
    imports: dict[Path, dict[str, str]] = field(
        default_factory=lambda: defaultdict(dict)
    )


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


class Slicer:
    """Slicer that extracts code snippets from files."""

    def __init__(self) -> None:
        """Initialize an empty slicer."""
        self.log = structlog.get_logger(__name__)

    def extract_snippets(  # noqa: C901
        self, files: list[File], language: str = "python"
    ) -> list[Snippet]:
        """Extract code snippets from a list of files.

        Args:
            files: List of domain File objects to analyze
            language: Programming language for analysis

        Returns:
            List of extracted code snippets as domain entities

        Raises:
            ValueError: If no files provided or language unsupported
            FileNotFoundError: If any file doesn't exist

        """
        if not files:
            raise ValueError("No files provided")

        language = language.lower()

        # Get language configuration
        if language not in LanguageConfig.CONFIGS:
            self.log.debug("Skipping", language=language)
            return []

        config = LanguageConfig.CONFIGS[language]

        # Initialize tree-sitter
        tree_sitter_name = self._get_tree_sitter_language_name(language)
        try:
            ts_language = get_language(tree_sitter_name)  # type: ignore[arg-type]
            parser = Parser(ts_language)
        except Exception as e:
            raise RuntimeError(f"Failed to load {language} parser: {e}") from e

        # Create mapping from Paths to File objects and extract paths
        path_to_file_map: dict[Path, File] = {}
        file_paths: list[Path] = []
        for file in files:
            file_path = file.as_path()

            # Validate file matches language
            if not self._file_matches_language(file_path.suffix, language):
                raise ValueError(f"File {file_path} does not match language {language}")

            # Validate file exists
            if not file_path.exists():
                raise FileNotFoundError(f"File not found: {file_path}")

            path_to_file_map[file_path] = file
            file_paths.append(file_path)

        # Initialize state
        state = AnalyzerState(parser=parser)
        state.files = file_paths
        file_contents: dict[Path, str] = {}

        # Parse all files
        for file_path in file_paths:
            try:
                with file_path.open("rb") as f:
                    source_code = f.read()
                tree = state.parser.parse(source_code)
                state.asts[file_path] = tree
            except OSError:
                # Skip files that can't be parsed
                continue

        # Build indexes
        self._build_definition_and_import_indexes(state, config, language)
        self._build_call_graph(state, config)
        self._build_reverse_call_graph(state)

        # Extract snippets for all functions
        snippets = []
        for qualified_name in state.def_index:
            snippet_content = self._get_snippet(
                qualified_name,
                state,
                file_contents,
                {"max_depth": 2, "max_functions": 8},
            )
            if "not found" not in snippet_content:
                snippet = self._create_snippet_entity(
                    qualified_name, snippet_content, language, state, path_to_file_map
                )
                snippets.append(snippet)

        return snippets

    def _file_matches_language(self, file_extension: str, language: str) -> bool:
        """Check if a file extension matches the current language."""
        if language not in LanguageConfig.CONFIGS:
            return False

        try:
            return (
                language == LanguageMapping.get_language_for_extension(file_extension)
            )
        except ValueError:
            # Extension not supported, so it doesn't match any language
            return False

    def _get_tree_sitter_language_name(self, language: str) -> str:
        """Map user language names to tree-sitter language names."""
        mapping = {
            "c++": "cpp",
            "c": "c",
            "cpp": "cpp",
            "java": "java",
            "rust": "rust",
            "python": "python",
            "go": "go",
            "javascript": "javascript",
            "typescript": "typescript",
            "js": "javascript",
            "ts": "typescript",
            "csharp": "csharp",
            "c#": "csharp",
            "cs": "csharp",
            "html": "html",
            "css": "css",
        }
        return mapping.get(language, language)

    def _build_definition_and_import_indexes(
        self, state: AnalyzerState, config: dict[str, Any], language: str
    ) -> None:
        """Build definition and import indexes."""
        for file_path, tree in state.asts.items():
            # Build definition index
            for node in self._walk_tree(tree.root_node):
                if self._is_function_definition(node, config):
                    qualified_name = self._qualify_name(
                        node, file_path, config, language
                    )
                    if qualified_name:
                        span = (node.start_byte, node.end_byte)
                        state.def_index[qualified_name] = FunctionInfo(
                            file=file_path,
                            node=node,
                            span=span,
                            qualified_name=qualified_name,
                        )

            # Build import map
            file_imports = {}
            for node in self._walk_tree(tree.root_node):
                if self._is_import_statement(node, config):
                    imports = self._extract_imports(node)
                    file_imports.update(imports)
            state.imports[file_path] = file_imports

    def _build_call_graph(self, state: AnalyzerState, config: dict[str, Any]) -> None:
        """Build call graph from function definitions."""
        for qualified_name, func_info in state.def_index.items():
            calls = self._find_function_calls(
                func_info.node, func_info.file, state, config
            )
            state.call_graph[qualified_name] = calls

    def _build_reverse_call_graph(self, state: AnalyzerState) -> None:
        """Build reverse call graph."""
        for caller, callees in state.call_graph.items():
            for callee in callees:
                state.reverse_calls[callee].add(caller)

    def _walk_tree(self, node: Node) -> Generator[Node, None, None]:
        """Walk the AST tree, yielding all nodes."""
        # Use a simple queue-based approach to avoid recursion issues
        queue = [node]
        visited: set[int] = set()  # Track by node id (memory address)

        while queue:
            current = queue.pop(0)

            # Use node id (memory address) as unique identifier to avoid infinite loops
            node_id = id(current)
            if node_id in visited:
                continue
            visited.add(node_id)

            yield current

            # Add children to queue
            queue.extend(current.children)

    def _is_function_definition(self, node: Node, config: dict[str, Any]) -> bool:
        """Check if node is a function definition."""
        return node.type in (config["function_nodes"] + config["method_nodes"])

    def _is_import_statement(self, node: Node, config: dict[str, Any]) -> bool:
        """Check if node is an import statement."""
        return node.type in config["import_nodes"]

    def _extract_function_name(
        self, node: Node, config: dict[str, Any], language: str
    ) -> str | None:
        """Extract function name from a function definition node."""
        if language == "html":
            return self._extract_html_element_name(node)
        if language == "css":
            return self._extract_css_rule_name(node)
        if language == "go" and node.type == "method_declaration":
            return self._extract_go_method_name(node)
        if language in ["c", "cpp"] and config["name_field"]:
            return self._extract_c_cpp_function_name(node, config)
        if language == "rust" and config["name_field"]:
            return self._extract_rust_function_name(node, config)
        return self._extract_default_function_name(node)

    def _extract_go_method_name(self, node: Node) -> str | None:
        """Extract method name from Go method declaration."""
        for child in node.children:
            if child.type == "field_identifier" and child.text is not None:
                return child.text.decode("utf-8")
        return None

    def _extract_c_cpp_function_name(
        self, node: Node, config: dict[str, Any]
    ) -> str | None:
        """Extract function name from C/C++ function definition."""
        declarator = node.child_by_field_name(config["name_field"])
        if not declarator:
            return None

        if declarator.type == "function_declarator":
            for child in declarator.children:
                if child.type == "identifier" and child.text is not None:
                    return child.text.decode("utf-8")
        elif declarator.type == "identifier" and declarator.text is not None:
            return declarator.text.decode("utf-8")
        return None

    def _extract_rust_function_name(
        self, node: Node, config: dict[str, Any]
    ) -> str | None:
        """Extract function name from Rust function definition."""
        name_node = node.child_by_field_name(config["name_field"])
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
                    return "".join(selector_parts[:2])  # First couple selectors
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

    def _qualify_name(
        self, node: Node, file_path: Path, config: dict[str, Any], language: str
    ) -> str | None:
        """Create qualified name for a function node."""
        function_name = self._extract_function_name(node, config, language)
        if not function_name:
            return None

        module_name = file_path.stem
        return f"{module_name}.{function_name}"

    def _get_file_content(self, file_path: Path, file_contents: dict[Path, str]) -> str:
        """Get cached file content."""
        if file_path not in file_contents:
            try:
                with file_path.open(encoding="utf-8") as f:
                    file_contents[file_path] = f.read()
            except UnicodeDecodeError as e:
                file_contents[file_path] = f"# Error reading file: {e}"
            except OSError as e:
                file_contents[file_path] = f"# Error reading file: {e}"
        return file_contents[file_path]

    def _get_snippet(
        self,
        function_name: str,
        state: AnalyzerState,
        file_contents: dict[Path, str],
        snippet_config: dict[str, Any] | None = None,
    ) -> str:
        """Generate a smart snippet for a function with its dependencies."""
        if snippet_config is None:
            snippet_config = {}

        max_depth = snippet_config.get("max_depth", 2)
        max_functions = snippet_config.get("max_functions", 8)
        include_usage = snippet_config.get("include_usage", True)

        if function_name not in state.def_index:
            return f"Error: Function '{function_name}' not found"

        # Find dependencies
        dependencies = self._find_dependencies(
            function_name, state, max_depth, max_functions
        )

        # Sort dependencies topologically
        sorted_deps = self._topological_sort(dependencies, state)

        # Build snippet
        snippet_lines = []

        # Add imports
        imports = self._get_minimal_imports({function_name}.union(dependencies))
        if imports:
            snippet_lines.extend(imports)
            snippet_lines.append("")

        # Add target function
        target_source = self._extract_function_source(
            function_name, state, file_contents
        )
        snippet_lines.append(target_source)

        # Add dependencies
        if dependencies:
            snippet_lines.append("")
            snippet_lines.append("# === DEPENDENCIES ===")
            for dep in sorted_deps:
                snippet_lines.append("")
                dep_source = self._extract_function_source(dep, state, file_contents)
                snippet_lines.append(dep_source)

        # Add usage examples
        if include_usage:
            callers = state.reverse_calls.get(function_name, set())
            if callers:
                snippet_lines.append("")
                snippet_lines.append("# === USAGE EXAMPLES ===")
                for caller in list(callers)[:2]:  # Show up to 2 examples
                    call_line = self._find_function_call_line(
                        caller, function_name, state, file_contents
                    )
                    if call_line and not call_line.startswith("#"):
                        snippet_lines.append(f"# From {caller}:")
                        snippet_lines.append(f"    {call_line}")
                        snippet_lines.append("")

        return "\n".join(snippet_lines)

    def _create_snippet_entity(
        self,
        qualified_name: str,
        snippet_content: str,
        language: str,
        state: AnalyzerState,
        path_to_file_map: dict[Path, File],
    ) -> Snippet:
        """Create a Snippet domain entity from extracted content."""
        # Determine all files that this snippet derives from
        derives_from_files = self._find_source_files_for_snippet(
            qualified_name, snippet_content, state, path_to_file_map
        )

        # Create the snippet entity
        snippet = Snippet(derives_from=derives_from_files)

        # Add the original content
        snippet.add_original_content(snippet_content, language)

        return snippet

    def _find_source_files_for_snippet(
        self,
        qualified_name: str,
        snippet_content: str,
        state: AnalyzerState,
        path_to_file_map: dict[Path, File],
    ) -> list[File]:
        """Find all source files that a snippet derives from."""
        source_files: list[File] = []
        source_file_paths: set[Path] = set()

        # Add the primary function's file
        if qualified_name in state.def_index:
            primary_file_path = state.def_index[qualified_name].file
            if (
                primary_file_path in path_to_file_map
                and primary_file_path not in source_file_paths
            ):
                source_files.append(path_to_file_map[primary_file_path])
                source_file_paths.add(primary_file_path)

        # Find all dependencies mentioned in the snippet and add their source files
        dependencies = self._extract_dependency_names_from_snippet(
            snippet_content, state
        )
        for dep_name in dependencies:
            if dep_name in state.def_index:
                dep_file_path = state.def_index[dep_name].file
                if (
                    dep_file_path in path_to_file_map
                    and dep_file_path not in source_file_paths
                ):
                    source_files.append(path_to_file_map[dep_file_path])
                    source_file_paths.add(dep_file_path)

        return source_files

    def _extract_dependency_names_from_snippet(
        self, snippet_content: str, state: AnalyzerState
    ) -> set[str]:
        """Extract dependency function names from snippet content."""
        dependencies: set[str] = set()

        # Look for the DEPENDENCIES section and extract function names
        lines = snippet_content.split("\n")
        in_dependencies_section = False

        for original_line in lines:
            line = original_line.strip()
            if line == "# === DEPENDENCIES ===":
                in_dependencies_section = True
                continue
            if line == "# === USAGE EXAMPLES ===":
                in_dependencies_section = False
                continue

            if in_dependencies_section and line.startswith("def "):
                # Extract function name from "def function_name(...)" pattern
                func_def_start = line.find("def ") + 4
                func_def_end = line.find("(", func_def_start)
                if func_def_end > func_def_start:
                    func_name = line[func_def_start:func_def_end].strip()
                    # Try to find the qualified name (module.function_name format)
                    # We need to search through the state.def_index to find matches
                    for qualified_name in self._get_qualified_names_for_function(
                        func_name, state
                    ):
                        dependencies.add(qualified_name)

        return dependencies

    def _get_qualified_names_for_function(
        self, func_name: str, state: AnalyzerState
    ) -> list[str]:
        """Get possible qualified names for a function name."""
        # This is a simple implementation - in practice you might want more
        # sophisticated matching
        return [
            qualified
            for qualified in state.def_index
            if qualified.endswith(f".{func_name}")
        ]

    # Helper methods

    def _extract_imports(self, node: Node) -> dict[str, str]:
        """Extract imports from import node."""
        imports = {}
        if node.type == "import_statement":
            for child in node.children:
                if child.type == "dotted_name" and child.text is not None:
                    module_name = child.text.decode("utf-8")
                    imports[module_name] = module_name
        elif node.type == "import_from_statement":
            module_node = node.child_by_field_name("module_name")
            if module_node and module_node.text is not None:
                module_name = module_node.text.decode("utf-8")
                for child in node.children:
                    if child.type == "import_list":
                        for import_child in child.children:
                            if (
                                import_child.type == "dotted_name"
                                and import_child.text is not None
                            ):
                                imported_name = import_child.text.decode("utf-8")
                                imports[imported_name] = (
                                    f"{module_name}.{imported_name}"
                                )
        return imports

    def _find_function_calls(
        self, node: Node, file_path: Path, state: AnalyzerState, config: dict[str, Any]
    ) -> set[str]:
        """Find function calls in a node."""
        calls = set()
        call_node_type = config["call_node"]

        for child in self._walk_tree(node):
            if child.type == call_node_type:
                function_node = child.child_by_field_name("function")
                if function_node:
                    call_name = self._extract_call_name(function_node)
                    if call_name:
                        resolved = self._resolve_call(call_name, file_path, state)
                        if resolved:
                            calls.add(resolved)
        return calls

    def _extract_call_name(self, node: Node) -> str | None:
        """Extract function name from call node."""
        if node.type == "identifier" and node.text is not None:
            return node.text.decode("utf-8")
        if node.type == "attribute":
            object_node = node.child_by_field_name("object")
            attribute_node = node.child_by_field_name("attribute")
            if (
                object_node
                and attribute_node
                and object_node.text is not None
                and attribute_node.text is not None
            ):
                obj_name = object_node.text.decode("utf-8")
                attr_name = attribute_node.text.decode("utf-8")
                return f"{obj_name}.{attr_name}"
        return None

    def _resolve_call(
        self, call_name: str, file_path: Path, state: AnalyzerState
    ) -> str | None:
        """Resolve a function call to qualified name."""
        module_name = file_path.stem
        local_qualified = f"{module_name}.{call_name}"

        if local_qualified in state.def_index:
            return local_qualified

        # Check imports
        if file_path in state.imports:
            imports = state.imports[file_path]
            if call_name in imports:
                return imports[call_name]

        # Check if already qualified
        if call_name in state.def_index:
            return call_name

        return None

    def _find_dependencies(
        self, target: str, state: AnalyzerState, max_depth: int, max_functions: int
    ) -> set[str]:
        """Find relevant dependencies for a function."""
        visited: set[str] = set()
        to_visit = [(target, 0)]
        dependencies: set[str] = set()

        while to_visit and len(dependencies) < max_functions:
            current, depth = to_visit.pop(0)
            if current in visited or depth > max_depth:
                continue
            visited.add(current)

            if current != target:
                dependencies.add(current)

            # Add direct dependencies
            to_visit.extend(
                (callee, depth + 1)
                for callee in state.call_graph.get(current, set())
                if callee not in visited and callee in state.def_index
            )

        return dependencies

    def _topological_sort(self, functions: set[str], state: AnalyzerState) -> list[str]:
        """Sort functions in dependency order."""
        if not functions:
            return []

        # Build subgraph
        in_degree: dict[str, int] = defaultdict(int)
        graph: dict[str, set[str]] = defaultdict(set)

        for func in functions:
            for callee in state.call_graph.get(func, set()):
                if callee in functions:
                    graph[func].add(callee)
                    in_degree[callee] += 1

        # Find roots
        queue = [f for f in functions if in_degree[f] == 0]
        result = []

        while queue:
            current = queue.pop(0)
            result.append(current)
            for neighbor in graph[current]:
                in_degree[neighbor] -= 1
                if in_degree[neighbor] == 0:
                    queue.append(neighbor)

        # Add any remaining (cycles)
        for func in functions:
            if func not in result:
                result.append(func)

        return result

    def _get_minimal_imports(self, _functions: set[str]) -> list[str]:
        """Get minimal imports needed for functions."""
        # For now, we'll skip imports to simplify the refactoring
        return []

    def _extract_function_source(
        self, qualified_name: str, state: AnalyzerState, file_contents: dict[Path, str]
    ) -> str:
        """Extract complete function source code."""
        if qualified_name not in state.def_index:
            return f"# Function {qualified_name} not found"

        func_info = state.def_index[qualified_name]
        file_content = self._get_file_content(func_info.file, file_contents)

        # Extract function source using byte positions
        start_byte, end_byte = func_info.span
        source_bytes = file_content.encode("utf-8")
        return source_bytes[start_byte:end_byte].decode("utf-8")

    def _find_function_call_line(
        self,
        caller_qualified_name: str,
        target_name: str,
        state: AnalyzerState,
        file_contents: dict[Path, str],
    ) -> str:
        """Find the actual line where a function calls another."""
        if caller_qualified_name not in state.def_index:
            return f"# calls {target_name}"

        caller_info = state.def_index[caller_qualified_name]
        file_content = self._get_file_content(caller_info.file, file_contents)
        source_bytes = file_content.encode("utf-8")

        # Extract the caller function source
        start_byte, end_byte = caller_info.span
        function_source = source_bytes[start_byte:end_byte].decode("utf-8")

        # Look for lines that contain the target function call
        lines = function_source.split("\n")
        target_simple_name = target_name.split(".")[-1]  # Get just the function name

        for line in lines:
            if target_simple_name in line and "(" in line:
                # Clean up the line (remove leading/trailing whitespace)
                clean_line = line.strip()
                if clean_line:
                    return clean_line

        return f"# calls {target_name}"
