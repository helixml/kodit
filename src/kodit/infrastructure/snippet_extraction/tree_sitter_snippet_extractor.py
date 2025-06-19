"""Infrastructure implementation using tree-sitter for method extraction."""

from pathlib import Path
from typing import cast

from tree_sitter import Node, Query
from tree_sitter_language_pack import SupportedLanguage, get_language, get_parser

from kodit.domain.services.snippet_extraction_service import SnippetExtractor
from kodit.infrastructure.snippet_extraction.snippet_query_provider import (
    SnippetQueryProvider,
)


class TreeSitterSnippetExtractor(SnippetExtractor):
    """Infrastructure implementation using tree-sitter for method extraction."""

    def __init__(self, query_provider: SnippetQueryProvider) -> None:
        """Initialize the tree-sitter snippet extractor.

        Args:
            query_provider: Provider for snippet queries

        """
        self.query_provider = query_provider

    async def extract(self, file_path: Path, language: str) -> list[str]:
        """Extract snippets using tree-sitter parsing.

        Args:
            file_path: Path to the file to extract snippets from
            language: The programming language of the file

        Returns:
            List of extracted code snippets

        Raises:
            ValueError: If the file cannot be read or language is not supported

        """
        try:
            # Get the query for the language
            query = await self.query_provider.get_query(language)
        except FileNotFoundError as e:
            raise ValueError(f"Unsupported language: {file_path}") from e

        # Get parser and language for tree-sitter
        try:
            tree_sitter_language = get_language(cast("SupportedLanguage", language))
            parser = get_parser(cast("SupportedLanguage", language))
        except Exception as e:
            raise ValueError(f"Unsupported language: {file_path}") from e

        # Create query object
        query_obj = Query(tree_sitter_language, query)

        # Read file content
        try:
            file_bytes = file_path.read_bytes()
        except Exception as e:
            raise ValueError(f"Failed to read file: {file_path}") from e

        # Parse and extract snippets
        tree = parser.parse(file_bytes)
        captures_by_name = query_obj.captures(tree.root_node)
        lines = file_bytes.decode().splitlines()

        # Extract snippets using the existing logic
        snippets = self._extract_snippets_from_captures(captures_by_name, lines)

        # If there are no results, return the entire file
        if not snippets:
            return [file_bytes.decode()]

        return snippets

    def _extract_snippets_from_captures(
        self, captures_by_name: dict[str, list[Node]], lines: list[str]
    ) -> list[str]:
        """Extract snippets from tree-sitter captures.

        Args:
            captures_by_name: Captures organized by name
            lines: Lines of the source file

        Returns:
            List of extracted code snippets

        """
        # Find all leaf functions
        leaf_functions = self._get_leaf_functions(captures_by_name)

        # Find all imports
        imports = self._get_imports(captures_by_name)

        results = []

        # For each leaf function, find all lines this function is dependent on
        for func_node in leaf_functions:
            all_lines_to_keep = set()

            ancestors = self._get_ancestors(captures_by_name, func_node)

            # Add self to keep
            all_lines_to_keep.update(
                range(func_node.start_point[0], func_node.end_point[0] + 1)
            )

            # Add imports to keep
            for import_node in imports:
                all_lines_to_keep.update(
                    range(import_node.start_point[0], import_node.end_point[0] + 1)
                )

            # Add ancestors to keep
            for node in ancestors:
                # Get the first line of the node for now
                start = node.start_point[0]
                end = node.start_point[0]
                all_lines_to_keep.update(range(start, end + 1))

            pseudo_code = []
            for i, line in enumerate(lines):
                if i in all_lines_to_keep:
                    pseudo_code.append(line)

            results.append("\n".join(pseudo_code))

        return results

    def _get_leaf_functions(
        self, captures_by_name: dict[str, list[Node]]
    ) -> list[Node]:
        """Return all leaf functions in the AST."""
        return [
            node
            for node in captures_by_name.get("function.body", [])
            if self._is_leaf_function(captures_by_name, node)
        ]

    def _is_leaf_function(
        self, captures_by_name: dict[str, list[Node]], node: Node
    ) -> bool:
        """Return True if the node is a leaf function."""
        for other in captures_by_name.get("function.body", []):
            if other == node:  # Skip self
                continue
            # if other is inside node, it's not a leaf function
            if other.start_byte >= node.start_byte and other.end_byte <= node.end_byte:
                return False
        return True

    def _get_imports(self, captures_by_name: dict[str, list[Node]]) -> list[Node]:
        """Return all imports in the AST."""
        return captures_by_name.get("import.name", []) + captures_by_name.get(
            "import.from", []
        )

    def _classes_and_functions(
        self, captures_by_name: dict[str, list[Node]]
    ) -> list[int]:
        """Return all classes and functions in the AST."""
        return [
            node.id
            for node in {
                *captures_by_name.get("function.def", []),
                *captures_by_name.get("class.def", []),
            }
        ]

    def _get_ancestors(
        self, captures_by_name: dict[str, list[Node]], node: Node
    ) -> list[Node]:
        """Return all ancestors of the node."""
        valid_ancestors = self._classes_and_functions(captures_by_name)
        ancestors = []
        parent = node.parent
        while parent:
            if parent.id in valid_ancestors:
                ancestors.append(parent)
            parent = parent.parent
        return ancestors
