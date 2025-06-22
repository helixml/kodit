import pytest
from tree_sitter import Node as ASTNode

from src.kodit.infrastructure.slicing.ast_parser import ASTParser
from src.kodit.infrastructure.slicing.cfg import CFGBuilder
from src.kodit.infrastructure.slicing.pdg import PDGBuilder
from src.kodit.infrastructure.slicing.sdg import SDGBuilder
from src.kodit.infrastructure.slicing.slicer import Slicer, SlicingCriterion
from src.kodit.infrastructure.slicing.reconstruction import (
    CodeReconstructor,
)


@pytest.fixture
def sample_code() -> str:
    """A sample Python code snippet for slicing tests."""
    return """
def foo(a, b):
    z = 10  # This should NOT be in the slice
    if a > 0:
        x = b + 1
    else:
        x = b - 1
    y = x * 2
    return y
"""


def test_end_to_end_slicing(sample_code: str):
    """
    Tests an end-to-end backward slice.
    It builds the SDG, slices from the `return y` statement, and verifies that
    only the statements that affect `y` are included in the slice.
    """
    # 1. Setup: Parse the code and build the graph hierarchy
    language = "python"
    parser = ASTParser(language)
    tree = parser.parse(sample_code.encode("utf-8"))
    root_node = parser.get_root_node(tree)

    cfg = CFGBuilder(language).build(root_node)
    pdg = PDGBuilder(cfg).build()
    sdg = SDGBuilder(pdg).build(call_graph=None)

    # 2. Slicing Criterion: Find the 'return y' node in the SDG
    slicing_criterion_node = None
    for node in sdg.nodes:
        if node.data and node.data.type == "return_statement":
            slicing_criterion_node = node
            break
    assert slicing_criterion_node, "Slicing criterion 'return y' not found in SDG"

    criterion = SlicingCriterion({slicing_criterion_node})

    # 3. Perform Slice
    slicer = Slicer(sdg)
    backward_slice = slicer.slice(criterion, backward=True)

    # 4. Assertions
    # Find the 'z = 10' assignment node in the original AST
    z_assignment_node_ast = None
    for node in ASTParser.traverse(root_node):
        if node.type == "expression_statement" and "z = 10" in ASTParser.get_node_text(
            node
        ):
            z_assignment_node_ast = node
            break
    assert z_assignment_node_ast, "Could not find 'z = 10' AST node"

    # Find the corresponding SDG node
    z_sdg_node = None
    for n in sdg.nodes:
        if n.data == z_assignment_node_ast:
            z_sdg_node = n
            break
    assert z_sdg_node, "Could not find SDG node for 'z = 10'"

    # The node for 'z = 10' should not be in the slice
    assert z_sdg_node not in backward_slice, (
        "Node for 'z = 10' should not be in the slice"
    )

    # Check that the slice contains a reasonable number of nodes.
    assert len(backward_slice) > 3, "Slice seems too small"
    assert len(backward_slice) < 15, "Slice seems too large"


def test_slice_reconstruction(sample_code: str):
    """
    Tests that a slice can be reconstructed back into valid, partial code.
    """
    # 1. Setup: Parse the code and build the graph hierarchy
    language = "python"
    parser = ASTParser(language)
    tree = parser.parse(sample_code.encode("utf-8"))
    root_node = parser.get_root_node(tree)

    cfg = CFGBuilder(language).build(root_node)
    pdg = PDGBuilder(cfg).build()
    sdg = SDGBuilder(pdg).build(call_graph=None)

    # 2. Slicing Criterion: Find the 'return y' node in the SDG
    slicing_criterion_node = None
    for node in sdg.nodes:
        if node.data and node.data.type == "return_statement":
            slicing_criterion_node = node
            break
    assert slicing_criterion_node, "Slicing criterion 'return y' not found in SDG"

    criterion = SlicingCriterion({slicing_criterion_node})

    # 3. Perform Slice
    slicer = Slicer(sdg)
    backward_slice = slicer.slice(criterion, backward=True)

    # 4. Reconstruct Code from Slice
    reconstructor = CodeReconstructor(sample_code, root_node)
    reconstructed_code = reconstructor.reconstruct(backward_slice)

    # 5. Assertions
    assert "def foo(a, b):" in reconstructed_code
    assert "z = 10" not in reconstructed_code
    assert "if a > 0:" in reconstructed_code
    assert "x = b + 1" in reconstructed_code
    assert "else:" in reconstructed_code
    assert "x = b - 1" in reconstructed_code
    assert "y = x * 2" in reconstructed_code
    assert "return y" in reconstructed_code

    # Check for proper indentation and structure
    expected_code = """
def foo(a, b):
    if a > 0:
        x = b + 1
    else:
        x = b - 1
    y = x * 2
    return y
""".strip()

    assert reconstructed_code.strip() == expected_code
