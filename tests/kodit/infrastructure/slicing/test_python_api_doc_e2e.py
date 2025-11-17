"""End-to-end test for Python API documentation generation."""

import tempfile
from datetime import UTC, datetime
from pathlib import Path

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.api_doc_extractor import APIDocExtractor


def test_python_class_with_constructor_and_method() -> None:
    """Test Python class with constructor parameter and method parameter.

    This test verifies the end-to-end flow of:
    1. Parsing a Python class with a constructor that has parameters
    2. Parsing a method with parameters
    3. Generating Pydoc-Markdown style documentation
    4. Verifying the output format includes inline constructor parameters
    """
    # Create a test Python file with a simple class
    python_code = '''"""Calculator module for basic math operations."""


class Calculator:
    """A simple calculator class.

    This calculator performs basic arithmetic operations.
    """

    def __init__(
        self,
        precision: int = 2,
    ):
        """Initialize the calculator.

        Args:
            precision: Number of decimal places for results
        """
        self.precision = precision

    def add(
        self,
        a: float,
        b: float,
    ) -> float:
        """Add two numbers together.

        Args:
            a: First number
            b: Second number

        Returns:
            The sum of a and b
        """
        return round(a + b, self.precision)
'''

    # Write to a temporary file
    with tempfile.NamedTemporaryFile(mode="w", suffix=".py", delete=False) as temp_file:
        temp_file.write(python_code)
        temp_path = temp_file.name

    try:
        # Create GitFile for the test file
        git_file = GitFile(
            created_at=datetime.now(tz=UTC),
            blob_sha="test_calculator_sha",
            commit_sha="test_commit_sha",
            path=temp_path,
            mime_type="text/x-python",
            size=Path(temp_path).stat().st_size,
            extension=".py",
        )

        # Extract API docs
        extractor = APIDocExtractor()
        enrichments = extractor.extract_api_docs([git_file], "python")

        # Verify we got exactly one enrichment
        assert len(enrichments) == 1, f"Expected 1 enrichment, got {len(enrichments)}"

        enrichment = enrichments[0]
        content = enrichment.content

        # Print the generated documentation for inspection
        print("\n" + "=" * 80)  # noqa: T201
        print("GENERATED PYTHON API DOCUMENTATION:")  # noqa: T201
        print("=" * 80)  # noqa: T201
        print(content)  # noqa: T201
        print("=" * 80 + "\n")  # noqa: T201

        # Verify enrichment metadata
        assert enrichment.type == "usage"
        assert enrichment.subtype == "api_docs"
        assert enrichment.language == "python"

        # Verify the documentation structure
        assert "# python API Reference" in content
        assert "## " in content  # Should have module section

        # Verify the class is documented
        assert "Calculator" in content

        # Verify constructor parameters appear inline (Pydoc-Markdown style)
        # The class signature should include constructor params
        assert "precision: int = 2" in content

        # Verify the class docstring is present
        assert "A simple calculator class" in content

        # Verify the method is documented
        assert "add" in content
        assert "#### Methods" in content or "###" in content

        # Verify method parameters are in signature
        assert "a: float" in content
        assert "b: float" in content

        # Verify method docstring
        assert "Add two numbers together" in content

        # Verify code fences use Python syntax highlighting
        assert "```py" in content or "```python" in content

        # Verify __init__ is NOT shown as a separate method
        assert "__init__" not in content or content.count("__init__") == 0

    finally:
        # Clean up temporary file
        Path(temp_path).unlink()


def test_python_class_multiple_methods() -> None:
    """Test Python class with multiple methods to verify complete documentation."""
    python_code = '''"""Math utilities module."""


class MathUtils:
    """Mathematical utility functions."""

    def __init__(self, base: int, multiplier: float = 1.0):
        """Initialize math utilities.

        Args:
            base: Base value for calculations
            multiplier: Multiplier to apply to results
        """
        self.base = base
        self.multiplier = multiplier

    def square(self, x: int) -> int:
        """Calculate the square of a number.

        Args:
            x: Number to square

        Returns:
            The square of x
        """
        return int((x ** 2) * self.multiplier)

    def cube(self, x: int) -> int:
        """Calculate the cube of a number.

        Args:
            x: Number to cube

        Returns:
            The cube of x
        """
        return int((x ** 3) * self.multiplier)
'''

    with tempfile.NamedTemporaryFile(mode="w", suffix=".py", delete=False) as temp_file:
        temp_file.write(python_code)
        temp_path = temp_file.name

    try:
        git_file = GitFile(
            created_at=datetime.now(tz=UTC),
            blob_sha="test_mathutils_sha",
            commit_sha="test_commit_sha",
            path=temp_path,
            mime_type="text/x-python",
            size=Path(temp_path).stat().st_size,
            extension=".py",
        )

        extractor = APIDocExtractor()
        enrichments = extractor.extract_api_docs([git_file], "python")

        assert len(enrichments) == 1
        content = enrichments[0].content

        # Print for inspection
        print("\n" + "=" * 80)  # noqa: T201
        print("GENERATED PYTHON API DOCUMENTATION (Multiple Methods):")  # noqa: T201
        print("=" * 80)  # noqa: T201
        print(content)  # noqa: T201
        print("=" * 80 + "\n")  # noqa: T201

        # Verify class with inline constructor params
        assert "MathUtils" in content
        assert "base: int" in content
        assert "multiplier: float = 1.0" in content

        # Verify both methods are documented
        assert "square" in content
        assert "cube" in content

        # Verify method signatures include parameters
        assert "x: int" in content

        # Verify docstrings
        assert "Calculate the square of a number" in content
        assert "Calculate the cube of a number" in content

    finally:
        Path(temp_path).unlink()
