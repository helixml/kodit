"""Complete self-contained analyzer for kodit-slicer.

This module combines all necessary functionality without external dependencies
on the legacy domain/application/infrastructure layers.
"""

from collections import defaultdict
from collections.abc import Generator
from dataclasses import dataclass, field
from pathlib import Path
from typing import TYPE_CHECKING, Any

import structlog
from tree_sitter import Node, Parser, Tree

from kodit.domain.entities.git import GitFile, SnippetV2
from kodit.domain.value_objects import LanguageMapping
from kodit.infrastructure.slicing.ast_analyzer import ASTAnalyzer
from kodit.infrastructure.slicing.code_elements import FunctionInfo
from kodit.infrastructure.slicing.language_analyzer import language_analyzer_factory

if TYPE_CHECKING:
    from kodit.infrastructure.slicing.code_elements import (
        FunctionDefinition,
        ParsedFile,
        TypeDefinition,
    )


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


class Slicer:
    """Slicer that extracts code snippets from files."""

    def __init__(self) -> None:
        """Initialize an empty slicer."""
        self.log = structlog.get_logger(__name__)

    def extract_snippets_from_git_files(  # noqa: C901
        self, files: list[GitFile], language: str = "python"
    ) -> list[SnippetV2]:
        """Extract code snippets from a list of files.

        Args:
            files: List of domain File objects to analyze
            language: Programming language for analysis

        Returns:
            List of extracted code snippets as domain entities

        Raises:
            FileNotFoundError: If any file doesn't exist

        """
        if not files:
            return []

        language = language.lower()

        # Initialize ASTAnalyzer
        try:
            analyzer = ASTAnalyzer(language)
        except ValueError:
            self.log.debug("Skipping unsupported language", language=language)
            return []

        # Validate files
        path_to_file_map: dict[Path, GitFile] = {}
        for file in files:
            file_path = Path(file.path)

            # Validate file matches language
            if not self._file_matches_language(file_path.suffix, language):
                raise ValueError(f"File {file_path} does not match language {language}")

            # Validate file exists
            if not file_path.exists():
                raise FileNotFoundError(f"File not found: {file_path}")

            path_to_file_map[file_path] = file

        # Parse files and extract definitions using ASTAnalyzer
        parsed_files = analyzer.parse_files(files)
        if not parsed_files:
            return []

        functions, _, types = analyzer.extract_definitions(
            parsed_files, include_private=True
        )

        # Build state from ASTAnalyzer results
        state = self._build_state_from_ast_analyzer(parsed_files, functions)
        lang_analyzer = language_analyzer_factory(language)

        # Build type lookup by simple name for prepending to function snippets
        type_lookup: dict[str, TypeDefinition] = {t.simple_name: t for t in types}

        # Build call graph and snippets (Slicer-specific logic)
        self._build_call_graph(state, lang_analyzer)
        self._build_reverse_call_graph(state)

        # Extract snippets for all functions
        file_contents: dict[Path, str] = {}
        snippets: list[SnippetV2] = []
        for qualified_name in state.def_index:
            snippet_content = self._get_snippet(
                qualified_name,
                state,
                file_contents,
                {"max_depth": 2, "max_functions": 8},
            )
            if "not found" not in snippet_content:
                # Prepend referenced types for TypeScript/TSX
                if language in ("typescript", "ts", "tsx"):
                    snippet_content = self._prepend_referenced_types(
                        qualified_name,
                        snippet_content,
                        state,
                        type_lookup,
                        file_contents,
                        lang_analyzer,
                    )

                snippet = self._create_snippet_entity_from_git_files(
                    qualified_name, snippet_content, language, state, path_to_file_map
                )
                snippets.append(snippet)

        # Extract snippets for types (interfaces, type aliases)
        for type_def in types:
            type_snippet = self._create_type_snippet(
                type_def, language, path_to_file_map, file_contents
            )
            if type_snippet:
                snippets.append(type_snippet)

        # Extract JSX snippets from functions with large JSX returns
        jsx_snippets = self._extract_jsx_snippets(
            state, language, path_to_file_map, file_contents
        )
        snippets.extend(jsx_snippets)

        # Extract top-level entry point expressions (e.g., ReactDOM.render calls)
        entry_point_snippets = self._extract_entry_point_snippets(
            parsed_files, language, path_to_file_map, file_contents
        )
        snippets.extend(entry_point_snippets)

        return snippets

    def _file_matches_language(self, file_extension: str, language: str) -> bool:
        """Check if a file extension matches the current language."""
        try:
            language_analyzer_factory(language)
        except ValueError:
            return False

        try:
            return language == LanguageMapping.get_language_for_extension(
                file_extension
            )
        except ValueError:
            # Extension not supported, so it doesn't match any language
            return False

    def _build_state_from_ast_analyzer(
        self,
        parsed_files: list["ParsedFile"],
        functions: list["FunctionDefinition"],
    ) -> AnalyzerState:
        """Build AnalyzerState from ASTAnalyzer results."""
        # Create a dummy parser (not used for new parsing)
        from tree_sitter_language_pack import get_language

        ts_language = get_language("python")
        parser = Parser(ts_language)

        state = AnalyzerState(parser=parser)

        # Populate files and ASTs from ParsedFile objects
        for parsed in parsed_files:
            state.files.append(parsed.path)
            state.asts[parsed.path] = parsed.tree

        # Populate def_index from FunctionDefinition objects
        for func_def in functions:
            state.def_index[func_def.qualified_name] = FunctionInfo(
                file=func_def.file,
                node=func_def.node,
                span=func_def.span,
                qualified_name=func_def.qualified_name,
            )

        return state

    def _build_call_graph(self, state: AnalyzerState, analyzer: Any) -> None:
        """Build call graph from function definitions."""
        for qualified_name, func_info in state.def_index.items():
            calls = self._find_function_calls(
                func_info.node, func_info.file, state, analyzer
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

        # Filter out nested functions (they're already separate snippets)
        func_info = state.def_index[function_name]
        nested_func_names = {
            name
            for name, info in state.def_index.items()
            if (
                info.file == func_info.file
                and info.span[0] > func_info.span[0]
                and info.span[1] <= func_info.span[1]
            )
        }
        dependencies = dependencies - nested_func_names

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

        # Add dependencies (excluding nested functions)
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
                # Show up to 2 examples, sorted for deterministic order
                for caller in sorted(callers)[:2]:
                    call_line = self._find_function_call_line(
                        caller, function_name, state, file_contents
                    )
                    if call_line and not call_line.startswith("#"):
                        snippet_lines.append(f"# From {caller}:")
                        snippet_lines.append(f"    {call_line}")
                        snippet_lines.append("")

        return "\n".join(snippet_lines)

    def _create_snippet_entity_from_git_files(
        self,
        qualified_name: str,
        snippet_content: str,
        language: str,
        state: AnalyzerState,
        path_to_file_map: dict[Path, GitFile],
    ) -> SnippetV2:
        """Create a Snippet domain entity from extracted content."""
        # Determine all files that this snippet derives from
        derives_from_files = self._find_source_files_for_snippet_from_git_files(
            qualified_name, snippet_content, state, path_to_file_map
        )

        # Create the snippet entity
        return SnippetV2(
            derives_from=derives_from_files,
            content=snippet_content,
            extension=language,
            sha=SnippetV2.compute_sha(snippet_content),
        )

    def _create_type_snippet(
        self,
        type_def: "TypeDefinition",
        language: str,
        path_to_file_map: dict[Path, GitFile],
        file_contents: dict[Path, str],
    ) -> SnippetV2 | None:
        """Create a snippet from a type definition (interface, type alias)."""
        # Get the source content for this type
        file_content = self._get_file_content(type_def.file, file_contents)
        source_bytes = file_content.encode("utf-8")
        start_byte, end_byte = type_def.span
        type_source = source_bytes[start_byte:end_byte].decode("utf-8")

        if not type_source.strip():
            return None

        # Find the source file
        derives_from: list[GitFile] = []
        if type_def.file in path_to_file_map:
            derives_from.append(path_to_file_map[type_def.file])

        return SnippetV2(
            derives_from=derives_from,
            content=type_source,
            extension=language,
            sha=SnippetV2.compute_sha(type_source),
        )

    def _extract_jsx_snippets(
        self,
        state: AnalyzerState,
        language: str,
        path_to_file_map: dict[Path, GitFile],
        file_contents: dict[Path, str],
    ) -> list[SnippetV2]:
        """Extract JSX return statements as separate snippets."""
        jsx_snippets: list[SnippetV2] = []
        min_lines = 5  # Only extract JSX returns with at least this many lines

        for qualified_name, func_info in state.def_index.items():
            file_content = self._get_file_content(func_info.file, file_contents)
            source_bytes = file_content.encode("utf-8")
            start_byte, end_byte = func_info.span
            func_source = source_bytes[start_byte:end_byte].decode("utf-8")

            # Look for JSX return statements
            jsx_content = self._extract_jsx_return_content(func_source, min_lines)
            if jsx_content:
                # Create snippet for the JSX
                simple_name = qualified_name.split(".")[-1]
                jsx_header = f"// JSX from {simple_name}\n"

                derives_from: list[GitFile] = []
                if func_info.file in path_to_file_map:
                    derives_from.append(path_to_file_map[func_info.file])

                jsx_snippets.append(
                    SnippetV2(
                        derives_from=derives_from,
                        content=jsx_header + jsx_content,
                        extension=language,
                        sha=SnippetV2.compute_sha(jsx_header + jsx_content),
                    )
                )

        return jsx_snippets

    def _extract_entry_point_snippets(
        self,
        parsed_files: list["ParsedFile"],
        language: str,
        path_to_file_map: dict[Path, GitFile],
        file_contents: dict[Path, str],
    ) -> list[SnippetV2]:
        """Extract top-level entry point expressions (e.g., ReactDOM.render calls)."""
        entry_snippets: list[SnippetV2] = []

        for parsed in parsed_files:
            file_content = self._get_file_content(parsed.path, file_contents)
            source_bytes = file_content.encode("utf-8")

            # Look for top-level expression statements
            for child in parsed.tree.root_node.children:
                if child.type == "expression_statement":
                    # Check if this is a meaningful entry point (render call, etc.)
                    expr_text = source_bytes[child.start_byte : child.end_byte].decode(
                        "utf-8"
                    )

                    # Skip trivial expressions (single identifiers, etc.)
                    if self._is_entry_point_expression(expr_text):
                        content = expr_text

                        derives_from: list[GitFile] = []
                        if parsed.path in path_to_file_map:
                            derives_from.append(path_to_file_map[parsed.path])

                        entry_snippets.append(
                            SnippetV2(
                                derives_from=derives_from,
                                content=content,
                                extension=language,
                                sha=SnippetV2.compute_sha(content),
                            )
                        )

        return entry_snippets

    def _is_entry_point_expression(self, expr_text: str) -> bool:
        """Check if an expression statement is a meaningful entry point."""
        import re

        # Check for common entry point patterns
        entry_patterns = [
            r"ReactDOM\.(createRoot|render)",  # React entry points
            r"render\s*\(",  # Generic render calls
            r"mount\s*\(",  # Vue/other framework mounts
            r"bootstrap\s*\(",  # Bootstrap patterns
            r"createApp\s*\(",  # Vue 3
        ]

        for pattern in entry_patterns:
            if re.search(pattern, expr_text):
                return True

        # Also check if expression contains JSX (indicates UI entry point)
        return bool(re.search(r"<\w+", expr_text))

    def _extract_jsx_return_content(
        self, func_source: str, min_lines: int = 5
    ) -> str | None:
        """Extract the JSX content from a return statement if it's large enough."""
        import re

        lines = func_source.split("\n")
        i = 0

        while i < len(lines):
            line = lines[i]
            stripped = line.strip()

            # Check if this is the start of a return statement
            if stripped.startswith("return (") or stripped == "return (":
                # Find the matching closing paren
                paren_count = line.count("(") - line.count(")")
                return_lines = [line]

                j = i + 1
                while j < len(lines) and paren_count > 0:
                    return_lines.append(lines[j])
                    paren_count += lines[j].count("(") - lines[j].count(")")
                    j += 1

                # Check if return contains JSX (has < tag)
                return_content = "\n".join(return_lines)
                has_jsx = bool(re.search(r"<\w+", return_content))

                # If it's a large JSX return, extract it
                if has_jsx and len(return_lines) >= min_lines:
                    return return_content

            i += 1

        return None

    def _find_source_files_for_snippet_from_git_files(
        self,
        qualified_name: str,
        snippet_content: str,
        state: AnalyzerState,
        path_to_file_map: dict[Path, GitFile],
    ) -> list[GitFile]:
        """Find all source files that a snippet derives from."""
        source_files: list[GitFile] = []
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
        self, node: Node, file_path: Path, state: AnalyzerState, analyzer: Any
    ) -> set[str]:
        """Find function calls in a node."""
        calls = set()
        call_node_type = analyzer.node_types().call_node

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
                for callee in sorted(state.call_graph.get(current, set()))
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

        for func in sorted(functions):
            for callee in sorted(state.call_graph.get(func, set())):
                if callee in functions:
                    graph[func].add(callee)
                    in_degree[callee] += 1

        # Find roots
        queue = [f for f in sorted(functions) if in_degree[f] == 0]
        result = []

        while queue:
            current = queue.pop(0)
            result.append(current)
            for neighbor in sorted(graph[current]):
                in_degree[neighbor] -= 1
                if in_degree[neighbor] == 0:
                    queue.append(neighbor)

        # Add any remaining (cycles)
        for func in sorted(functions):
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
        """Extract complete function source code, summarizing nested functions."""
        if qualified_name not in state.def_index:
            return f"# Function {qualified_name} not found"

        func_info = state.def_index[qualified_name]
        file_content = self._get_file_content(func_info.file, file_contents)
        source_bytes = file_content.encode("utf-8")

        # Extract function source using byte positions
        start_byte, end_byte = func_info.span
        func_source = source_bytes[start_byte:end_byte].decode("utf-8")

        # Find nested functions that are contained within this function's span
        nested_functions = self._find_nested_functions(
            func_info, state, source_bytes, start_byte
        )

        # If there are nested functions, summarize them
        if nested_functions:
            func_source = self._summarize_nested_functions(
                func_source, nested_functions, start_byte
            )

        # Summarize large JSX return statements
        return self._summarize_jsx_return(func_source)


    def _find_nested_functions(
        self,
        parent_func: FunctionInfo,
        state: AnalyzerState,
        source_bytes: bytes,
        parent_start: int,
    ) -> list[tuple[str, int, int, str]]:
        """Find functions nested within a parent function.

        Returns list of (name, relative_start, relative_end, signature) tuples.
        """
        parent_start_byte, parent_end_byte = parent_func.span
        nested = []

        for name, info in state.def_index.items():
            # Skip self
            if name == parent_func.qualified_name:
                continue

            # Check if this function is nested within the parent
            if (
                info.file == parent_func.file
                and info.span[0] > parent_start_byte
                and info.span[1] <= parent_end_byte
            ):
                # Get the signature (first line up to the opening brace or arrow)
                func_bytes = source_bytes[info.span[0] : info.span[1]]
                func_text = func_bytes.decode("utf-8")
                signature = self._extract_function_signature(func_text)

                # Store relative positions within parent function
                rel_start = info.span[0] - parent_start
                rel_end = info.span[1] - parent_start
                nested.append((name.split(".")[-1], rel_start, rel_end, signature))

        # Sort by position (descending) so we can replace from end to start
        nested.sort(key=lambda x: x[1], reverse=True)
        return nested

    def _extract_function_signature(self, func_text: str) -> str:
        """Extract the signature part of a function (before the body)."""
        # For arrow functions: const foo = (args) => { ... }
        # We want: const foo = (args) =>
        if "=>" in func_text:
            arrow_pos = func_text.find("=>")
            return func_text[: arrow_pos + 2].strip()

        # For regular functions: function foo(args) { ... }
        # We want: function foo(args)
        if "{" in func_text:
            brace_pos = func_text.find("{")
            return func_text[:brace_pos].strip()

        return func_text.split("\n")[0].strip()

    def _summarize_nested_functions(
        self,
        func_source: str,
        nested_functions: list[tuple[str, int, int, str]],
        _parent_start: int,
    ) -> str:
        """Replace nested function bodies with '{ ... }' placeholders."""
        result = func_source

        for _name, rel_start, rel_end, signature in nested_functions:
            # Replace the entire nested function with just its signature + ...
            nested_text = result[rel_start:rel_end]

            # Preserve the indentation
            line_start = result.rfind("\n", 0, rel_start)
            if line_start != -1:
                line_content = result[line_start + 1 : rel_start]
                line_content[: len(line_content) - len(line_content.lstrip())]

            # Check if it ends with semicolon (for const declarations)
            ends_with_semi = nested_text.rstrip().endswith(";")
            suffix = ";" if ends_with_semi else ""

            # Create summarized version
            summarized = f"{signature} {{ ... }}{suffix}"
            result = result[:rel_start] + summarized + result[rel_end:]

        return result

    def _summarize_jsx_return(self, func_source: str, min_lines: int = 5) -> str:
        """Summarize large JSX return statements to 'return ( ... );'."""
        import re

        # Look for return statements with JSX (parenthesized expressions with <)
        # Pattern: return ( ... ); or return ( ... )
        # We need to handle multi-line returns with JSX

        lines = func_source.split("\n")
        result_lines = []
        i = 0

        while i < len(lines):
            line = lines[i]
            stripped = line.strip()

            # Check if this is the start of a return statement
            if stripped.startswith("return (") or stripped == "return (":
                # Find the matching closing paren
                paren_count = line.count("(") - line.count(")")
                return_lines = [line]

                j = i + 1
                while j < len(lines) and paren_count > 0:
                    return_lines.append(lines[j])
                    paren_count += lines[j].count("(") - lines[j].count(")")
                    j += 1

                # Check if return contains JSX (has < tag)
                return_content = "\n".join(return_lines)
                has_jsx = bool(re.search(r"<\w+", return_content))

                # If it's a large JSX return, summarize it
                if has_jsx and len(return_lines) >= min_lines:
                    # Get the indentation from the return line
                    indent = line[: len(line) - len(line.lstrip())]
                    result_lines.append(f"{indent}return ( ... );")
                    i = j
                else:
                    result_lines.append(line)
                    i += 1
            else:
                result_lines.append(line)
                i += 1

        return "\n".join(result_lines)

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

    def _prepend_referenced_types(  # noqa: PLR0913
        self,
        qualified_name: str,
        snippet_content: str,
        state: AnalyzerState,
        type_lookup: dict[str, "TypeDefinition"],
        file_contents: dict[Path, str],
        lang_analyzer: Any,
    ) -> str:
        """Prepend referenced type definitions to a function snippet."""
        if qualified_name not in state.def_index:
            return snippet_content

        func_info = state.def_index[qualified_name]

        # Check if the analyzer supports type reference extraction
        if not hasattr(lang_analyzer, "extract_type_references"):
            return snippet_content

        # Extract type references from the function node
        type_refs = lang_analyzer.extract_type_references(func_info.node)
        if not type_refs:
            return snippet_content

        # Find matching type definitions
        type_sources: list[str] = []
        for type_name in sorted(type_refs):
            if type_name in type_lookup:
                type_def = type_lookup[type_name]
                type_source = self._get_type_source(type_def, file_contents)
                if type_source:
                    type_sources.append(type_source)

        if not type_sources:
            return snippet_content

        # Prepend types to snippet
        return "\n\n".join(type_sources) + "\n\n" + snippet_content

    def _get_type_source(
        self, type_def: "TypeDefinition", file_contents: dict[Path, str]
    ) -> str | None:
        """Get the source code for a type definition."""
        file_content = self._get_file_content(type_def.file, file_contents)
        source_bytes = file_content.encode("utf-8")
        start_byte, end_byte = type_def.span
        type_source = source_bytes[start_byte:end_byte].decode("utf-8")
        return type_source.strip() if type_source.strip() else None
