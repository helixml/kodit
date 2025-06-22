from tree_sitter import Node as ASTNode

from .cfg import CFG
from .graph import Edge, Graph, Node


class ControlDependenceEdge(Edge):
    pass


class DataDependenceEdge(Edge):
    def __init__(self, source: Node, destination: Node, variable: str):
        super().__init__(source, destination, label=f"data:{variable}")
        self.variable = variable


class PDG(Graph[ASTNode]):
    """Represents a Program Dependence Graph (PDG).
    The nodes are AST nodes, and edges are control or data dependencies.
    """


class PDGBuilder:
    """Builds a Program Dependence Graph from a Control Flow Graph."""

    def __init__(self, cfg: CFG):
        self.cfg = cfg

    def build(self) -> PDG:
        """Builds the Program Dependence Graph (PDG) from the given Control Flow Graph (CFG)."""
        pdg = PDG()

        # Add all nodes from CFG to PDG
        for node in self.cfg.nodes:
            pdg.add_node(node_data=node.data, node_id=node.id)

        # Add control dependence edges
        self._add_control_dependence_edges(pdg)

        # Add data dependence edges
        self._add_data_dependence_edges(pdg)

        return pdg

    def _add_control_dependence_edges(self, pdg: PDG):
        """Adds control dependence edges to the PDG.
        A node Y is control-dependent on a node X if X determines whether Y is executed.
        """
        for node in self.cfg.nodes:
            # Control dependence arises from nodes with multiple outgoing edges (e.g., if, while)
            outgoing_edges = [e for e in self.cfg.edges if e.source.id == node.id]
            if (
                node.data
                and node.data.type == "if_statement"
                and len(outgoing_edges) > 1
            ):
                # This is a simplified approach. A more robust solution would use a
                # post-dominator tree.
                for edge in outgoing_edges:
                    # The destination of the edge is control-dependent on the source
                    if pdg.get_node(node.id) and pdg.get_node(edge.destination.id):
                        # Add control dependence to the children of the if statement's branches
                        if edge.label in ("true", "false") and edge.destination.data:
                            for child in edge.destination.data.children:
                                child_node = self.cfg.add_node(child)
                                pdg.add_node(child_node.data, child_node.id)
                                pdg.add_edge(node.id, child_node.id, "control")
                        else:
                            pdg.add_edge(node.id, edge.destination.id, "control")
            elif len(outgoing_edges) > 1:
                for edge in outgoing_edges:
                    if pdg.get_node(node.id) and pdg.get_node(edge.destination.id):
                        pdg.add_edge(node.id, edge.destination.id, "control")

    def _add_data_dependence_edges(self, pdg: PDG):
        """Adds data dependence edges to the PDG.
        A node Y is data-dependent on a node X if X defines a variable that is used in Y.
        """
        # This is a simplified approach that connects every use to all definitions.
        # A more correct implementation would use reaching definitions analysis.
        defs: dict[str, list[Node[ASTNode]]] = {}
        for node in pdg.nodes:
            if not node.data:
                continue

            defines = self._find_defs(node.data)
            for var_name in defines:
                if var_name not in defs:
                    defs[var_name] = []
                defs[var_name].append(node)

        for use_node in pdg.nodes:
            if not use_node.data:
                continue

            uses = self._find_uses(use_node.data)
            for use in uses:
                if use in defs:
                    for def_node in defs[use]:
                        if def_node != use_node:
                            # We should check if the def reaches the use, but for now,
                            # we connect all defs to all uses of the same variable.
                            pdg.add_edge(def_node.id, use_node.id, f"data:{use}")

    def _find_uses(self, node: ASTNode) -> list[str]:
        """Finds the variables used in the given node."""
        uses = []

        def traverse(n: ASTNode):
            if n.type == "identifier":
                # This is a simplification. We should check the context of the identifier.
                # For example, in `foo.bar`, `foo` is a use, but `bar` is not.
                # In `def foo():`, `foo` is a def, not a use.
                if n.text:
                    # check if the parent is an assignment, and if this is the left hand side
                    if (
                        n.parent
                        and n.parent.type == "assignment"
                        and n.parent.child_by_field_name("left") == n
                    ):
                        return
                    uses.append(n.text.decode())

            for child in n.children:
                traverse(child)

        traverse(node)
        return uses

    def _find_defs(self, node: ASTNode) -> list[str]:
        """Finds the variables defined in the given node."""
        defs = []

        def traverse(n: ASTNode):
            if n.type == "assignment":
                left = n.child_by_field_name("left")
                if left and left.type == "identifier" and left.text:
                    defs.append(left.text.decode())
            elif n.type == "function_definition":
                name_node = n.child_by_field_name("name")
                if name_node and name_node.text:
                    defs.append(name_node.text.decode())

                params_node = n.child_by_field_name("parameters")
                if params_node:
                    for param in params_node.children:
                        if param.type == "identifier" and param.text:
                            defs.append(param.text.decode())

            for child in n.children:
                traverse(child)

        traverse(node)
        return defs
