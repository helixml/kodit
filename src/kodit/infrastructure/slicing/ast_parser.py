from pathlib import Path
from typing import cast

from tree_sitter import Language, Node, Parser, Tree
from tree_sitter_language_pack import SupportedLanguage, get_language


class ASTParser:
    """Parses source code into an Abstract Syntax Tree (AST)."""

    def __init__(self, language: str):
        self.ts_language: Language = get_language(cast("SupportedLanguage", language))
        self.parser = Parser()
        self.parser.language = self.ts_language

    @property
    def language(self) -> Language:
        return self.ts_language

    def parse(self, content: Path | bytes) -> Tree:
        """Parses the given content into a tree-sitter Tree.

        Args:
            content: The content to parse, either as a file path or bytes.

        Returns:
            The parsed tree-sitter Tree.

        """
        try:
            if isinstance(content, Path):
                file_bytes = content.read_bytes()
            else:
                file_bytes = content
            tree = self.parser.parse(file_bytes)
            return tree
        except Exception as e:
            raise ValueError("Failed to parse content") from e

    def get_root_node(self, tree: Tree) -> Node:
        """Returns the root node of the given tree."""
        return tree.root_node

    @staticmethod
    def get_node_text(node: Node) -> str:
        if node.text:
            return node.text.decode("utf-8")
        return ""

    @staticmethod
    def traverse(node: Node):
        """A generator to traverse the tree depth-first."""
        yield node
        for child in node.children:
            yield from ASTParser.traverse(child)
