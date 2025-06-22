from typing import TYPE_CHECKING

from tree_sitter import Node as ASTNode

from src.kodit.infrastructure.slicing.graph import Node as GraphNode

if TYPE_CHECKING:
    from tree_sitter import Node as ASTNode


class CodeReconstructor:
    """Reconstructs a code snippet from a program slice."""

    def __init__(self, original_code: str, root_node: ASTNode):
        self.original_code_lines = original_code.splitlines()
        self.root_node = root_node
        self.original_code_bytes = original_code.encode("utf-8")

    def reconstruct(self, program_slice: set[GraphNode[ASTNode]]) -> str:
        """Takes a set of GraphNodes (a slice) and reconstructs the corresponding
        source code.
        """
        slice_ast_nodes = {n.data for n in program_slice if n.data}

        reconstructed_lines = self._get_reconstructed_lines(slice_ast_nodes)

        # Smartly handle indentation
        return self._format_reconstructed_code(reconstructed_lines)

    def _get_reconstructed_lines(self, slice_ast_nodes: set[ASTNode]) -> list[str]:
        lines_to_include = set()

        statement_nodes = self._get_statement_nodes()

        # Map every line to its containing statement node
        line_to_statement_map = {}
        for stmt in statement_nodes:
            for i in range(stmt.start_point[0], stmt.end_point[0] + 1):
                # Smallest statement node will win
                if i not in line_to_statement_map or (
                    stmt.end_byte - stmt.start_byte
                    < line_to_statement_map[i].end_byte
                    - line_to_statement_map[i].start_byte
                ):
                    line_to_statement_map[i] = stmt

        for line_num, line in enumerate(self.original_code_lines):
            if not line.strip():
                continue

            containing_stmt = line_to_statement_map.get(line_num)
            if containing_stmt and containing_stmt in slice_ast_nodes:
                lines_to_include.add(line_num)

        # Add back structural lines like `else:`
        for node in slice_ast_nodes:
            if node.type == "if_statement":
                for child in node.children:
                    if child.type == "else_clause":
                        lines_to_include.add(child.start_point[0])

        if not lines_to_include:
            return []

        return [self.original_code_lines[i] for i in sorted(list(lines_to_include))]

    def _get_statement_nodes(self) -> list[ASTNode]:
        """Heuristically find all statement-level nodes.
        For Python, these are typically direct children of a `block` node.
        """
        stmts = []
        q = [self.root_node]
        visited = set()
        while q:
            node = q.pop(0)
            if node in visited:
                continue
            visited.add(node)

            if node.type == "block":
                stmts.extend(c for c in node.children if c.is_named)

            for child in node.children:
                q.append(child)

        # Add the function definition itself, it acts as a statement container
        if self.root_node.type == "module" and self.root_node.children:
            for child in self.root_node.children:
                if child.type == "function_definition":
                    stmts.append(child)

        return stmts

    def _format_reconstructed_code(self, lines: list[str]) -> str:
        if not lines:
            return ""

        # Smartly handle indentation by finding the minimum indentation
        # of the sliced code and removing it from all lines.
        min_indent = float("inf")
        for line in lines:
            if line.strip():
                indent = len(line) - len(line.lstrip())
                min_indent = min(min_indent, indent)

        return "\n".join(line[min_indent:] for line in lines)
