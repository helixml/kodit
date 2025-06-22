from tree_sitter import Node as ASTNode

from .call_graph import CallGraph
from .graph import Graph
from .pdg import PDG


class SDG(Graph[ASTNode]):
    """Represents a System Dependence Graph (SDG).
    It is constructed by connecting multiple PDGs (for each procedure)
    with interprocedural edges based on a call graph.
    """


class SDGBuilder:
    """Builds a System Dependence Graph from a collection of PDGs and a Call Graph."""

    def __init__(self, pdg: PDG):
        self.pdg = pdg

    def build(self, call_graph: CallGraph | None) -> SDG:
        """Builds the System Dependence Graph (SDG) from the given Program Dependence Graphs (PDGs)
        and Call Graph.
        """
        sdg = SDG()
        # For a single module, the SDG is just the PDG plus any call information.

        # Add all nodes from PDG to SDG
        for node in self.pdg.nodes:
            sdg.add_node(node.data, node.id)

        # Add all edges from PDG to SDG
        for edge in self.pdg.edges:
            sdg.add_edge(edge.source.id, edge.destination.id, edge.label)

        if call_graph:
            # Add call graph edges
            for edge in call_graph.edges:
                # Ensure nodes exist in SDG before adding edge
                if sdg.get_node(edge.source.id) and sdg.get_node(edge.destination.id):
                    sdg.add_edge(edge.source.id, edge.destination.id, "call")

        return sdg

    def _add_interprocedural_edges(self):
        # Placeholder for adding call and parameter edges.
        # For each call in the call graph from function A to B:
        # - Find the call site node in A's PDG.
        # - Find the entry node in B's PDG.
        # - Add a call edge.
        # - Handle parameters by adding parameter-in and parameter-out edges.
        pass

    def _compute_summary_edges(self):
        # Placeholder for computing summary edges.
        # This is a complex data-flow analysis problem on the SDG.
        pass
