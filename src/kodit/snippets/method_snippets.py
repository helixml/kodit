"""Extract method snippets from source code."""

from tree_sitter import Node, Query
from tree_sitter_language_pack import SupportedLanguage, get_language, get_parser


class MethodSnippets:
    """Extract method snippets from source code."""

    def __init__(self, language: SupportedLanguage, query: str) -> None:
        """Initialize the MethodSnippets class."""
        self.language = get_language(language)
        self.parser = get_parser(language)
        self.query = Query(self.language, query)

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

    def extract(self, source_code: bytes) -> list[str]:
        """Extract method snippets from source code."""
        tree = self.parser.parse(source_code)

        captures_by_name = self.query.captures(tree.root_node)

        lines = source_code.decode().splitlines()

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

        # If there are no results, then return the entire file
        if not results:
            return [source_code.decode()]

        return results
