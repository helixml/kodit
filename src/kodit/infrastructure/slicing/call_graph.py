from tree_sitter import Node as ASTNode

from .graph import Graph

# For now, a function is just represented by its name (a string).
# This could be a more complex object later.
Function = str


class CallGraph(Graph[Function]):
    """Represents a Call Graph.
    Nodes are functions, and an edge from A to B means A calls B.
    """


class CallGraphBuilder:
    def __init__(self, language: str):
        self.language = language
        self.call_graph = CallGraph()

    def build(self, ast_root: ASTNode) -> CallGraph:
        """Builds the call graph from an AST.
        This is language-specific and requires identifying function definitions
        and call expressions in the AST.
        """
        # Placeholder implementation.
        # A real implementation would:
        # 1. Find all function definitions in the AST. These are the nodes.
        # 2. For each function, find all call expressions inside it.
        # 3. For each call, resolve it to a function definition (this can be hard).
        # 4. Add an edge in the call graph.

        return self.call_graph
