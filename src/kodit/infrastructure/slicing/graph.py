import uuid
from typing import Generic, TypeVar

T = TypeVar("T")


class Node(Generic[T]):
    def __init__(self, node_data: T | None, node_id: str | None = None):
        self.id = node_id or str(uuid.uuid4())
        self.data = node_data

    def __repr__(self) -> str:
        return f"Node(id={self.id}, data={self.data})"

    def __eq__(self, other):
        if not isinstance(other, Node):
            return NotImplemented
        return self.id == other.id

    def __hash__(self):
        return hash(self.id)


class Edge(Generic[T]):
    def __init__(self, source: Node[T], destination: Node[T], label: str | None = None):
        self.source = source
        self.destination = destination
        self.label = label

    def __repr__(self) -> str:
        return f"Edge(source={self.source.id}, destination={self.destination.id}, label={self.label})"

    def __eq__(self, other):
        if not isinstance(other, Edge):
            return NotImplemented
        return (
            self.source == other.source
            and self.destination == other.destination
            and self.label == other.label
        )

    def __hash__(self):
        return hash((self.source, self.destination, self.label))


class Graph(Generic[T]):
    def __init__(self):
        self._nodes: dict[str, Node[T]] = {}
        self._edges: list[Edge[T]] = []

    @property
    def nodes(self) -> list[Node[T]]:
        return list(self._nodes.values())

    @property
    def edges(self) -> list[Edge[T]]:
        return self._edges

    def add_node(self, node_data: T | None, node_id: str | None = None) -> Node[T]:
        if node_id is None:
            node_id = str(uuid.uuid4())

        if node_id in self._nodes:
            # Potentially update data or just return existing node
            # For now, let's just return the existing node
            return self._nodes[node_id]

        node = Node(node_data, node_id)
        self._nodes[node_id] = node
        return node

    def get_node(self, node_id: str) -> Node[T] | None:
        return self._nodes.get(node_id)

    def add_edge(
        self, source_id: str, dest_id: str, label: str | None = None
    ) -> Edge[T]:
        source_node = self.get_node(source_id)
        dest_node = self.get_node(dest_id)

        if not source_node or not dest_node:
            raise ValueError("Source or destination node not in graph")

        edge = Edge(source_node, dest_node, label)
        self._edges.append(edge)
        return edge

    def get_outgoing_edges(self, node_id: str) -> list[Edge[T]]:
        return [edge for edge in self._edges if edge.source.id == node_id]

    def get_incoming_edges(self, node_id: str) -> list[Edge[T]]:
        return [edge for edge in self._edges if edge.destination.id == node_id]

    def __repr__(self) -> str:
        return f"Graph(nodes={len(self._nodes)}, edges={len(self._edges)})"
