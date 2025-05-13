"""Extract independent code blocks from Python source code."""

from collections.abc import Generator
from pathlib import Path

from tree_sitter import Node
from tree_sitter_language_pack import SupportedLanguage, get_parser


class MethodParser:
    """First version of code graph extraction.

    My thesis is that methods are a good block of functionality. So the goal of this
    class is to find all low-level methods, then build them back up into something that
    looks like a file, with imports and classes, but only include the method code.
    """

    def __init__(self, source_code: bytes, language: SupportedLanguage) -> None:
        """Initialize the parser."""
        self.source_code = source_code.decode("utf-8")
        self.parser = get_parser(language)
        self.tree = self.parser.parse(source_code)

    def extract(self) -> Generator[str, None, None]:
        """Extract all methods from the source code."""
        methods = self._extract_methods(self.tree.root_node)
        for block in methods:
            branch = self._append_ancestors(block)
            source_lines = self.source_code.split("\n")
            lines = []
            for line_no in self._line_numbers_in_branch(branch):
                lines.append(source_lines[line_no])
            yield "\n".join(lines)

    def _line_numbers_in_branch(self, branch: list[Node]) -> list[int]:
        """Get the line numbers of all the elements in the branch."""
        lines_in_branch = []
        for node in branch:
            if node.type == "class_definition":
                for i in range(
                    node.start_point[0], node.children[0].start_point[0] + 1
                ):
                    lines_in_branch.append(i)
            else:
                for i in range(node.start_point[0], node.end_point[0] + 1):
                    lines_in_branch.append(i)
        return sorted(set(lines_in_branch))

    def _extract_methods(self, node: Node) -> list[Node]:
        """Extract all methods from the AST."""
        methods = []
        if node.type in {"function_definition", "method_declaration"}:
            methods.append(node)
        for child in node.children:
            methods.extend(self._extract_methods(child))
        return methods

    def _append_ancestors(self, node: Node) -> list[Node]:
        """Prepend the node with all its ancestors."""
        parents = []
        if node.type in {"function_definition", "class_definition"}:
            parents.append(node)

        # Check if this level has any children that are import statements
        for child in node.children:
            if child.type in {"import_statement", "import_from_statement"}:
                parents.append(child)  # noqa: PERF401

        if node.parent is not None:
            parents.extend(self._append_ancestors(node.parent))
        return parents


if __name__ == "__main__":
    with Path("/Users/phil/code/helixml/kodit/tests/kodit/code_graph/csharp.cs").open(
        "rb"
    ) as f:
        source_code = f.read()
    parser = MethodParser(source_code, "csharp")
    extracted_methods = list(parser.extract())
    for method in extracted_methods:
        print(method)
        print("-" * 40)
