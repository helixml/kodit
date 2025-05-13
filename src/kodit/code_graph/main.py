from tree_sitter import Node
from tree_sitter_language_pack import get_parser
from typing import Dict, Set, List
from dataclasses import dataclass
from collections import defaultdict

@dataclass
class DependencyNode:
    name: str
    type: str  # 'function' or 'variable'
    dependencies: Set[str]
    defined_at: tuple[int, int]  # (line, column)

class DependencyGraph:
    def __init__(self):
        self.nodes: Dict[str, DependencyNode] = {}
        self.parser = get_parser("python")

    def parse_code(self, code: str) -> None:
        tree = self.parser.parse(bytes(code, "utf8"))
        self._process_node(tree.root_node)

    def _process_node(self, node: Node, current_function: str = None) -> None:
        if node.type == "function_definition":
            # Extract function name
            for child in node.children:
                if child.type == "identifier":
                    func_name = child.text.decode("utf8")
                    self.nodes[func_name] = DependencyNode(
                        name=func_name,
                        type="function",
                        dependencies=set(),
                        defined_at=(node.start_point[0], node.start_point[1])
                    )
                    current_function = func_name
                    break

        elif node.type == "identifier" and current_function:
            # Track variable usage within functions
            var_name = node.text.decode("utf8")
            if var_name in self.nodes:
                self.nodes[current_function].dependencies.add(var_name)

        # Process all children
        for child in node.children:
            self._process_node(child, current_function)

    def print_graph(self) -> None:
        print("\nDependency Graph:")
        print("===============")
        for name, node in self.nodes.items():
            print(f"\n{node.type.title()}: {name}")
            print(f"Defined at: Line {node.defined_at[0] + 1}, Column {node.defined_at[1] + 1}")
            if node.dependencies:
                print("Dependencies:")
                for dep in sorted(node.dependencies):
                    print(f"  - {dep}")
            else:
                print("No dependencies")

def main():
    # Example code with function dependencies
    code = """
import os

def get_pwd():
    return os.getcwd()

def main():
    print(get_pwd())

if __name__ == "__main__":
    main()
"""
    
    graph = DependencyGraph()
    graph.parse_code(code)
    graph.print_graph()

if __name__ == "__main__":
    main()