"""AST analyzer for extracting code definitions across multiple languages."""

from collections.abc import Generator
from pathlib import Path

import structlog
from tree_sitter import Node, Parser
from tree_sitter_language_pack import get_language

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.code_elements import (
    ClassDefinition,
    FunctionDefinition,
    ModuleDefinition,
    ParsedFile,
    TypeDefinition,
)
from kodit.infrastructure.slicing.language_analyzer import language_analyzer_factory


class ASTAnalyzer:
    """Language-agnostic AST analyzer using composition."""

    def __init__(self, language: str) -> None:
        """Initialize analyzer for a specific language."""
        self.language = language.lower()
        self.analyzer = language_analyzer_factory(self.language)
        ts_language = get_language(self.analyzer.metadata().tree_sitter_name)
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
                self.analyzer.extract_classes(parsed, include_private=include_private)
            )
            types.extend(
                self.analyzer.extract_types(parsed, include_private=include_private)
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
            constants: list[tuple[str, Node]] = []

            for parsed in module_files:
                functions.extend(
                    self._extract_functions(parsed, include_private=include_private)
                )
                classes.extend(
                    self.analyzer.extract_classes(
                        parsed, include_private=include_private
                    )
                )
                types.extend(
                    self.analyzer.extract_types(parsed, include_private=include_private)
                )
                # Constants extraction not implemented yet
                constants.extend([])

            module_doc = self.analyzer.extract_module_docstring(module_files)
            module_path = self.analyzer.extract_module_path(module_files[0])

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
        return node.type in self.analyzer.node_types().all_function_nodes

    def _qualify_name(self, node: Node, file_path: Path) -> str | None:
        """Create qualified name for a function node."""
        function_name = self.analyzer.extract_function_name(node)
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

                simple_name = self.analyzer.extract_function_name(node)
                if not simple_name:
                    continue

                is_public = self.analyzer.is_public(node, simple_name)
                if not include_private and not is_public:
                    continue

                span = (node.start_byte, node.end_byte)
                docstring = self.analyzer.extract_docstring(node)
                is_method = self.analyzer.is_method(node)

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
                        parameters=[],  # Not implemented yet
                        return_type=None,  # Not implemented yet
                    )
                )

        return functions

    def _group_by_module(
        self, parsed_files: list[ParsedFile]
    ) -> dict[str, list[ParsedFile]]:
        """Create one module per file."""
        modules: dict[str, list[ParsedFile]] = {}
        for idx, parsed in enumerate(parsed_files):
            unique_key = f"{parsed.path}#{idx}"
            modules[unique_key] = [parsed]
        return modules
