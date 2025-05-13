"""Extract independent code blocks from Python source code."""

from collections.abc import Generator
from pathlib import Path

from tree_sitter_language_pack import SupportedLanguage, get_language, get_parser


class QueryParser:
    """First version of code graph extraction.

    My thesis is that methods are a good block of functionality. So the goal of this
    class is to find all low-level methods, then build them back up into something that
    looks like a file, with imports and classes, but only include the method code.
    """

    def __init__(self, source_code: bytes, language: SupportedLanguage) -> None:
        """Initialize the parser."""
        self.source_code = source_code.decode("utf-8")
        self.parser = get_parser(language)
        self.language = get_language(language)
        self.tree = self.parser.parse(source_code)
        path = Path(__file__).parent.joinpath(
            "languages",
            f"{language}.scm",
        )
        self.query = path.read_text()

    def extract(self) -> Generator[str, None, None]:
        """Extract all methods from the source code."""
        query = self.language.query(self.query)
        captures = query.captures(self.tree.root_node)

        for tag, nodes in captures.items():
            if tag.startswith("name.definition"):
                for node in nodes:
                    cur_node = node
                    while cur_node.parent:
                        cur_node = cur_node.parent
                        if cur_node.start_point[0] != cur_node.end_point[0]:
                            break
                    yield cur_node.text.decode("utf-8") if cur_node.text else ""


if __name__ == "__main__":
    with Path("/Users/phil/code/helixml/kodit/tests/kodit/code_graph/python.py").open(
        "rb"
    ) as f:
        source_code = f.read()
    parser = QueryParser(source_code, "python")
    extracted_methods = list(parser.extract())
    for method in extracted_methods:
        print(method)
        print("-" * 40)
