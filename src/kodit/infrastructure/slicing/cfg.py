from tree_sitter import Node as ASTNode
from tree_sitter import Query

from .ast_parser import ASTParser
from .graph import Graph, Node
from .query_provider import QueryProvider


class CFG(Graph[ASTNode]):
    """Represents a Control Flow Graph (CFG).
    The nodes of this graph are AST nodes.
    """

    def __init__(self):
        super().__init__()
        self.entry_node: Node[ASTNode] | None = None
        self.exit_node: Node[ASTNode] | None = None


class CFGBuilder:
    """Builds a Control Flow Graph from an Abstract Syntax Tree."""

    def __init__(self, language: str):
        self.language = language
        self.parser = ASTParser(language)
        self.query_provider = QueryProvider()

    def build(self, root_node: ASTNode | None) -> CFG:
        """Builds the CFG from the given root AST node."""
        cfg = CFG()

        if not root_node:
            # Handle empty or invalid input
            entry_node = cfg.add_node(node_data=None, node_id="entry")
            exit_node = cfg.add_node(node_data=None, node_id="exit")
            cfg.add_edge(entry_node.id, exit_node.id, "empty")
            return cfg

        query_source = self.query_provider.get_query(self.language)
        query = Query(self.parser.language, query_source)
        captures = query.captures(root_node)

        statements = captures.get("statement", [])

        entry_node = cfg.add_node(node_data=root_node, node_id="entry")
        exit_node = cfg.add_node(node_data=root_node, node_id="exit")
        cfg.entry_node = entry_node
        cfg.exit_node = exit_node

        if not statements:
            cfg.add_edge(entry_node.id, exit_node.id, "empty")
            return cfg

        statement_nodes = [cfg.add_node(node_data=stmt) for stmt in statements]

        cfg.add_edge(entry_node.id, statement_nodes[0].id, "entry")

        for i in range(len(statement_nodes) - 1):
            source_node = statement_nodes[i]
            dest_node = statement_nodes[i + 1]

            if source_node.data and source_node.data.type == "if_statement":
                self._handle_if_statement(cfg, source_node, dest_node, captures)
            else:
                cfg.add_edge(source_node.id, dest_node.id, "next")

        last_node = statement_nodes[-1]
        if last_node.data and last_node.data.type != "if_statement":
            cfg.add_edge(last_node.id, exit_node.id, "exit")

        return cfg

    def _find_child_by_name(
        self, parent_node: ASTNode, name: str, captures: dict[str, list[ASTNode]]
    ) -> ASTNode | None:
        if name in captures:
            for node in captures[name]:
                if node.parent == parent_node:
                    return node
        return None

    def _handle_if_statement(
        self,
        cfg: "CFG",
        if_node: Node[ASTNode],
        next_node: Node[ASTNode],
        captures: dict[str, list[ASTNode]],
    ) -> None:
        if not if_node.data:
            return

        cond_node_ast = self._find_child_by_name(if_node.data, "if_condition", captures)
        true_branch_ast = self._find_child_by_name(
            if_node.data, "if_consequence", captures
        )
        false_branch_ast = self._find_child_by_name(
            if_node.data, "if_alternative", captures
        )

        if cond_node_ast:
            cond_node = cfg.add_node(cond_node_ast)
            cfg.add_edge(if_node.id, cond_node.id, "condition")

            true_node_added = False
            if true_branch_ast and true_branch_ast.children:
                # Assuming the first statement in the block is representative
                true_node = cfg.add_node(true_branch_ast.children[0])
                cfg.add_edge(cond_node.id, true_node.id, "true")
                cfg.add_edge(true_node.id, next_node.id, "next")
                true_node_added = True

            if false_branch_ast and false_branch_ast.children:
                # Assuming the first statement in the block is representative
                false_node = cfg.add_node(false_branch_ast.children[0])
                cfg.add_edge(cond_node.id, false_node.id, "false")
                cfg.add_edge(false_node.id, next_node.id, "next")
            elif true_node_added:
                # No 'else', so false branch goes to the next statement
                cfg.add_edge(cond_node.id, next_node.id, "false")

        else:
            # No condition, just flow through
            cfg.add_edge(if_node.id, next_node.id, "next")
