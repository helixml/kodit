"""Complete self-contained analyzer for kodit-slicer.

This module combines all necessary functionality without external dependencies
on the legacy domain/application/infrastructure layers.
"""

import re
from collections import defaultdict
from collections.abc import Generator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, ClassVar

from tree_sitter import Node, Parser, Tree
from tree_sitter_language_pack import get_language


@dataclass
class FunctionInfo:
    """Information about a function definition."""

    file: str
    node: Node
    span: tuple[int, int]
    qualified_name: str


@dataclass
class AnalyzerState:
    """Central state for the dependency analysis."""

    parser: Parser
    files: list[str] = field(default_factory=list)
    asts: dict[str, Tree] = field(default_factory=dict)
    def_index: dict[str, FunctionInfo] = field(default_factory=dict)
    call_graph: dict[str, set[str]] = field(default_factory=lambda: defaultdict(set))
    reverse_calls: dict[str, set[str]] = field(default_factory=lambda: defaultdict(set))
    imports: dict[str, dict[str, str]] = field(
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
    }

    # Aliases
    CONFIGS["c++"] = CONFIGS["cpp"]
    CONFIGS["typescript"] = CONFIGS["javascript"]
    CONFIGS["ts"] = CONFIGS["javascript"]
    CONFIGS["js"] = CONFIGS["javascript"]


class SimpleAnalyzer:
    """Simplified analyzer that directly provides all analysis functionality."""

    def __init__(self, root_path: str, language: str = "python") -> None:
        """Initialize analyzer with a codebase."""
        self.language = language.lower()
        self.root_path = root_path

        # Get language configuration
        if self.language not in LanguageConfig.CONFIGS:
            supported = ", ".join(sorted(LanguageConfig.CONFIGS.keys()))
            raise ValueError(
                f"Unsupported language: {language}. Supported languages: {supported}"
            )

        self.config = LanguageConfig.CONFIGS[self.language]

        # Initialize tree-sitter
        tree_sitter_name = self._get_tree_sitter_language_name()
        try:
            ts_language = get_language(tree_sitter_name)  # type: ignore[arg-type]
            parser = Parser(ts_language)
        except Exception as e:
            raise RuntimeError(f"Failed to load {language} parser: {e}") from e

        # Initialize state
        self.state = AnalyzerState(parser=parser)
        self._file_contents: dict[str, str] = {}  # Cache for file contents
        self._initialized = False

        # Initialize the pipeline
        self._discover_and_parse_files()

    def _get_tree_sitter_language_name(self) -> str:
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
        }
        return mapping.get(self.language, self.language)

    def _discover_and_parse_files(self) -> None:
        """Discover and parse all relevant files."""
        files = []
        root_path = Path(self.root_path)

        if not root_path.exists():
            raise FileNotFoundError(f"Directory not found: {self.root_path}")

        extension = self.config["extension"]

        # Handle single file case
        if root_path.is_file() and root_path.suffix == extension:
            files.append(str(root_path))
        else:
            # Recursively find all files with the extension
            files.extend(
                str(file_path)
                for file_path in root_path.rglob(f"*{extension}")
                if file_path.is_file()
            )

        self.state.files = files

        # Parse all files
        for file_path in files:
            try:
                with Path(file_path).open("rb") as f:
                    source_code = f.read()
                tree = self.state.parser.parse(source_code)
                self.state.asts[file_path] = tree
            except OSError:
                # Skip files that can't be parsed
                pass

    def _ensure_initialized(self) -> None:
        """Build indexes on first use."""
        if self._initialized:
            return

        self._build_definition_and_import_indexes()
        self._build_call_graph()
        self._build_reverse_call_graph()
        self._initialized = True

    def _build_definition_and_import_indexes(self) -> None:
        """Build definition and import indexes."""
        for file_path, tree in self.state.asts.items():
            # Build definition index
            for node in self._walk_tree(tree.root_node):
                if self._is_function_definition(node):
                    qualified_name = self._qualify_name(node, file_path)
                    if qualified_name:
                        span = (node.start_byte, node.end_byte)
                        self.state.def_index[qualified_name] = FunctionInfo(
                            file=file_path,
                            node=node,
                            span=span,
                            qualified_name=qualified_name,
                        )

            # Build import map
            file_imports = {}
            for node in self._walk_tree(tree.root_node):
                if self._is_import_statement(node):
                    imports = self._extract_imports(node)
                    file_imports.update(imports)
            self.state.imports[file_path] = file_imports

    def _build_call_graph(self) -> None:
        """Build call graph from function definitions."""
        for qualified_name, func_info in self.state.def_index.items():
            calls = self._find_function_calls(func_info.node, func_info.file)
            self.state.call_graph[qualified_name] = calls

    def _build_reverse_call_graph(self) -> None:
        """Build reverse call graph."""
        for caller, callees in self.state.call_graph.items():
            for callee in callees:
                self.state.reverse_calls[callee].add(caller)

    def _walk_tree(self, node: Node) -> Generator[Node, None, None]:
        """Walk the AST tree, yielding all nodes."""
        cursor = node.walk()

        def _walk_recursive() -> Generator[Node, None, None]:
            current_node = cursor.node
            if current_node is not None:
                yield current_node
            if cursor.goto_first_child():
                yield from _walk_recursive()
                while cursor.goto_next_sibling():
                    yield from _walk_recursive()
                cursor.goto_parent()

        yield from _walk_recursive()

    def _is_function_definition(self, node: Node) -> bool:
        """Check if node is a function definition."""
        return node.type in (
            self.config["function_nodes"] + self.config["method_nodes"]
        )

    def _is_import_statement(self, node: Node) -> bool:
        """Check if node is an import statement."""
        return node.type in self.config["import_nodes"]

    def _extract_function_name(self, node: Node) -> str | None:
        """Extract function name from a function definition node."""
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

    def _extract_default_function_name(self, node: Node) -> str | None:
        """Extract function name using default identifier search."""
        for child in node.children:
            if child.type == "identifier" and child.text is not None:
                return child.text.decode("utf-8")
        return None

    def _qualify_name(self, node: Node, file_path: str) -> str | None:
        """Create qualified name for a function node."""
        function_name = self._extract_function_name(node)
        if not function_name:
            return None

        module_name = Path(file_path).stem
        return f"{module_name}.{function_name}"

    def _get_file_content(self, file_path: str) -> str:
        """Get cached file content."""
        if file_path not in self._file_contents:
            try:
                with Path(file_path).open(encoding="utf-8") as f:
                    self._file_contents[file_path] = f.read()
            except OSError as e:
                self._file_contents[file_path] = f"# Error reading file: {e}"
        return self._file_contents[file_path]

    def get_functions(self, search_pattern: str | None = None) -> list[str]:
        """Get all functions, optionally filtered by search pattern."""
        self._ensure_initialized()

        functions = list(self.state.def_index.keys())

        if search_pattern:
            try:
                regex = re.compile(search_pattern, re.IGNORECASE)
                functions = [f for f in functions if regex.search(f)]
            except re.error:
                # Fallback to substring search
                pattern_lower = search_pattern.lower()
                functions = [f for f in functions if pattern_lower in f.lower()]

        return sorted(functions)

    def get_snippet(
        self,
        function_name: str,
        max_depth: int = 2,
        max_functions: int = 8,
        *,
        include_usage: bool = True,
    ) -> str:
        """Generate a smart snippet for a function with its dependencies."""
        self._ensure_initialized()

        if function_name not in self.state.def_index:
            return f"Error: Function '{function_name}' not found"

        # Find dependencies
        dependencies = self._find_dependencies(function_name, max_depth, max_functions)

        # Sort dependencies topologically
        sorted_deps = self._topological_sort(dependencies)

        # Build snippet
        snippet_lines = []

        # Add imports
        imports = self._get_minimal_imports({function_name}.union(dependencies))
        if imports:
            snippet_lines.extend(imports)
            snippet_lines.append("")

        # Add target function
        target_source = self._extract_function_source(function_name)
        snippet_lines.append(target_source)

        # Add dependencies
        if dependencies:
            snippet_lines.append("")
            snippet_lines.append("# === DEPENDENCIES ===")
            for dep in sorted_deps:
                snippet_lines.append("")
                dep_source = self._extract_function_source(dep)
                snippet_lines.append(dep_source)

        # Add usage examples
        if include_usage:
            callers = self.state.reverse_calls.get(function_name, set())
            if callers:
                snippet_lines.append("")
                snippet_lines.append("# === USAGE EXAMPLES ===")
                for caller in list(callers)[:2]:  # Show up to 2 examples
                    call_line = self._find_function_call_line(caller, function_name)
                    if call_line and not call_line.startswith("#"):
                        snippet_lines.append(f"# From {caller}:")
                        snippet_lines.append(f"    {call_line}")
                        snippet_lines.append("")

        return "\n".join(snippet_lines)

    def get_stats(self) -> dict[str, Any]:
        """Get codebase statistics."""
        self._ensure_initialized()

        total_functions = len(self.state.def_index)
        total_calls = sum(len(callees) for callees in self.state.call_graph.values())

        # Find most called function
        call_counts: dict[str, int] = defaultdict(int)
        for callees in self.state.call_graph.values():
            for callee in callees:
                call_counts[callee] += 1

        most_called = (
            max(call_counts.items(), key=lambda x: x[1]) if call_counts else ("none", 0)
        )

        return {
            "total_functions": total_functions,
            "total_calls": total_calls,
            "most_called": most_called,
        }

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
                            if (import_child.type == "dotted_name"
                                and import_child.text is not None):
                                imported_name = import_child.text.decode("utf-8")
                                imports[imported_name] = (
                                    f"{module_name}.{imported_name}"
                                )
        return imports

    def _find_function_calls(self, node: Node, file_path: str) -> set[str]:
        """Find function calls in a node."""
        calls = set()
        call_node_type = self.config["call_node"]

        for child in self._walk_tree(node):
            if child.type == call_node_type:
                function_node = child.child_by_field_name("function")
                if function_node:
                    call_name = self._extract_call_name(function_node)
                    if call_name:
                        resolved = self._resolve_call(call_name, file_path)
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
            if (object_node and attribute_node
                and object_node.text is not None
                and attribute_node.text is not None):
                obj_name = object_node.text.decode("utf-8")
                attr_name = attribute_node.text.decode("utf-8")
                return f"{obj_name}.{attr_name}"
        return None

    def _resolve_call(self, call_name: str, file_path: str) -> str | None:
        """Resolve a function call to qualified name."""
        module_name = Path(file_path).stem
        local_qualified = f"{module_name}.{call_name}"

        if local_qualified in self.state.def_index:
            return local_qualified

        # Check imports
        if file_path in self.state.imports:
            imports = self.state.imports[file_path]
            if call_name in imports:
                return imports[call_name]

        # Check if already qualified
        if call_name in self.state.def_index:
            return call_name

        return None

    def _find_dependencies(
        self, target: str, max_depth: int, max_functions: int
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
                for callee in self.state.call_graph.get(current, set())
                if callee not in visited and callee in self.state.def_index
            )

        return dependencies

    def _topological_sort(self, functions: set[str]) -> list[str]:
        """Sort functions in dependency order."""
        if not functions:
            return []

        # Build subgraph
        in_degree: dict[str, int] = defaultdict(int)
        graph: dict[str, set[str]] = defaultdict(set)

        for func in functions:
            for callee in self.state.call_graph.get(func, set()):
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

    def _get_minimal_imports(self, functions: set[str]) -> list[str]:
        """Get minimal imports needed for functions."""
        imports = set()

        for func_name in functions:
            if func_name in self.state.def_index:
                func_info = self.state.def_index[func_name]
                file_path = func_info.file

                if file_path in self.state.asts:
                    tree = self.state.asts[file_path]
                    for node in self._walk_tree(tree.root_node):
                        if (
                            self._is_import_statement(node)
                            and node.end_byte < func_info.node.start_byte
                            and node.text is not None
                        ):
                            import_text = node.text.decode("utf-8").strip()
                            imports.add(import_text)

        return sorted(imports)

    def _extract_function_source(self, qualified_name: str) -> str:
        """Extract complete function source code."""
        if qualified_name not in self.state.def_index:
            return f"# Function {qualified_name} not found"

        func_info = self.state.def_index[qualified_name]
        file_content = self._get_file_content(func_info.file)

        # Extract function source using byte positions
        start_byte, end_byte = func_info.span
        source_bytes = file_content.encode("utf-8")
        return source_bytes[start_byte:end_byte].decode("utf-8")

    def _find_function_call_line(
        self, caller_qualified_name: str, target_name: str
    ) -> str:
        """Find the actual line where a function calls another."""
        if caller_qualified_name not in self.state.def_index:
            return f"# calls {target_name}"

        caller_info = self.state.def_index[caller_qualified_name]
        file_content = self._get_file_content(caller_info.file)
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
