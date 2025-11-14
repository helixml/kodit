"""Tests for ASTAnalyzer."""

from datetime import UTC, datetime
from pathlib import Path

import pytest

from kodit.domain.entities.git import GitFile
from kodit.infrastructure.slicing.ast_analyzer import ASTAnalyzer


def test_ast_analyzer_initialization() -> None:
    """Test that ASTAnalyzer initializes correctly."""
    analyzer = ASTAnalyzer("python")
    assert analyzer.language == "python"
    assert analyzer.analyzer is not None


def test_ast_analyzer_unsupported_language() -> None:
    """Test that ASTAnalyzer raises error for unsupported language."""
    with pytest.raises(ValueError, match="Unsupported language"):
        ASTAnalyzer("invalid_lang")


def test_parse_files_with_python_file(tmp_path: Path) -> None:
    """Test parsing a simple Python file."""
    # Create a test Python file
    test_file = tmp_path / "test.py"
    test_file.write_text("def hello():\n    return 'world'\n")

    # Create GitFile entity
    git_file = GitFile(
        path=str(test_file),
        created_at=datetime(2024, 1, 1, tzinfo=UTC),
        blob_sha="test123",
        commit_sha="commit123",
        mime_type="text/x-python",
        size=100,
        extension=".py",
    )

    analyzer = ASTAnalyzer("python")
    parsed_files = analyzer.parse_files([git_file])

    assert len(parsed_files) == 1
    assert parsed_files[0].path == test_file
    assert parsed_files[0].tree is not None


def test_extract_definitions_from_python_file(tmp_path: Path) -> None:
    """Test extracting function definitions from Python file."""
    # Create a test Python file
    test_file = tmp_path / "test.py"
    test_file.write_text(
        """
def hello():
    return 'world'

def _private_func():
    return 'private'

def public_func():
    return 'public'
"""
    )

    git_file = GitFile(
        path=str(test_file),
        created_at=datetime(2024, 1, 1, tzinfo=UTC),
        blob_sha="test123",
        commit_sha="commit123",
        mime_type="text/x-python",
        size=100,
        extension=".py",
    )

    analyzer = ASTAnalyzer("python")
    parsed_files = analyzer.parse_files([git_file])
    functions, classes, types = analyzer.extract_definitions(parsed_files)

    # Should extract all functions including private when include_private=True
    assert len(functions) >= 3
    function_names = [f.simple_name for f in functions]
    assert "hello" in function_names
    assert "_private_func" in function_names
    assert "public_func" in function_names


def test_extract_definitions_filters_private(tmp_path: Path) -> None:
    """Test extracting only public functions."""
    test_file = tmp_path / "test.py"
    test_file.write_text(
        """
def hello():
    return 'world'

def _private_func():
    return 'private'
"""
    )

    git_file = GitFile(
        path=str(test_file),
        created_at=datetime(2024, 1, 1, tzinfo=UTC),
        blob_sha="test123",
        commit_sha="commit123",
        mime_type="text/x-python",
        size=100,
        extension=".py",
    )

    analyzer = ASTAnalyzer("python")
    parsed_files = analyzer.parse_files([git_file])
    functions, _, _ = analyzer.extract_definitions(parsed_files, include_private=False)

    function_names = [f.simple_name for f in functions]
    assert "hello" in function_names
    assert "_private_func" not in function_names
