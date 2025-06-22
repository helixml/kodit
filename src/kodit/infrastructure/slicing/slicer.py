from tree_sitter import Node as ASTNode

from .graph import Node
from .sdg import SDG


class SlicingCriterion:
    """Defines the slicing criterion, which is a set of SDG nodes.
    For example, a variable at a specific line number.
    """

    def __init__(self, nodes: set[Node[ASTNode]]):
        self.nodes = nodes


class Slicer:
    """Performs program slicing on a System Dependence Graph (SDG)."""

    def __init__(self, sdg: SDG):
        self.sdg = sdg

    def slice(
        self, criterion: SlicingCriterion, backward: bool = True
    ) -> set[Node[ASTNode]]:
        """Performs a program slice based on the given criterion.

        Args:
            criterion: The slicing criterion.
            backward: If True, performs a backward slice. Otherwise, a forward slice.

        Returns:
            A set of SDG nodes that are part of the slice.

        """
        sliced_nodes: set[Node[ASTNode]] = set()
        worklist: list[Node[ASTNode]] = list(criterion.nodes)

        while worklist:
            current_node = worklist.pop(0)
            if current_node in sliced_nodes:
                continue

            sliced_nodes.add(current_node)

            if backward:
                # Traverse backward along incoming edges
                edges = self.sdg.get_incoming_edges(current_node.id)
                for edge in edges:
                    if edge.source not in sliced_nodes:
                        worklist.append(edge.source)
            else:
                # Traverse forward along outgoing edges
                edges = self.sdg.get_outgoing_edges(current_node.id)
                for edge in edges:
                    if edge.destination not in sliced_nodes:
                        worklist.append(edge.destination)

        return sliced_nodes
