"""End-to-end tests."""

import os
import shutil
import tempfile
from pathlib import Path
from typing import Generator

import pytest
from click.testing import CliRunner

from kodit.cli import cli


@pytest.fixture
def runner() -> CliRunner:
    """Create a CliRunner instance."""
    return CliRunner()


@pytest.fixture
def test_repo() -> Generator[Path, None, None]:
    """Create a temporary test repository with some sample code."""
    # Create a temporary directory
    temp_dir = Path(tempfile.mkdtemp())

    # Create a sample Python file
    sample_file = temp_dir / "sample.py"
    sample_file.write_text("""
def hello_world():
    \"\"\"A simple hello world function.\"\"\"
    return "Hello, World!"

def add_numbers(a: int, b: int) -> int:
    \"\"\"Add two numbers together.\"\"\"
    return a + b
""")

    # Create a sample README
    readme = temp_dir / "README.md"
    readme.write_text(
        "# Test Repository\n\nThis is a test repository for kodit e2e tests."
    )

    yield temp_dir

    # Cleanup
    shutil.rmtree(temp_dir)


@pytest.fixture
def test_env() -> Generator[None, None, None]:
    """Set up test environment variables."""
    # Store original environment
    original_env = dict(os.environ)

    # Set test environment variables
    os.environ["KODIT_DISABLE_TELEMETRY"] = "true"
    os.environ["KODIT_LOG_LEVEL"] = "ERROR"

    yield

    # Restore original environment
    os.environ.clear()
    os.environ.update(original_env)


def test_source_management(runner: CliRunner, test_repo: Path, test_env: None) -> None:
    """Test source management commands."""
    # Test creating a source
    result = runner.invoke(cli, ["sources", "create", str(test_repo)])
    assert result.exit_code == 0
    assert "Source created:" in result.output

    # Test listing sources
    result = runner.invoke(cli, ["sources", "list"])
    assert result.exit_code == 0
    assert str(test_repo) in result.output


def test_index_management(runner: CliRunner, test_repo: Path, test_env: None) -> None:
    """Test index management commands."""
    # Create a source first
    runner.invoke(cli, ["sources", "create", str(test_repo)])

    # Test creating an index
    result = runner.invoke(cli, ["indexes", "create", "1"])
    assert result.exit_code == 0
    assert "Index created:" in result.output

    # Test listing indexes
    result = runner.invoke(cli, ["indexes", "list"])
    assert result.exit_code == 0
    assert "ID" in result.output
    assert "Created At" in result.output

    # Test running an index
    result = runner.invoke(cli, ["indexes", "run", "1"])
    assert result.exit_code == 0


def test_retrieval(runner: CliRunner, test_repo: Path, test_env: None) -> None:
    """Test retrieval functionality."""
    # Set up source and index
    runner.invoke(cli, ["sources", "create", str(test_repo)])
    runner.invoke(cli, ["indexes", "create", "1"])
    runner.invoke(cli, ["indexes", "run", "1"])

    # Test retrieval
    result = runner.invoke(cli, ["retrieve", "hello world function"])
    assert result.exit_code == 0
    assert "hello_world" in result.output
    assert "Hello, World!" in result.output


def test_version_command(runner: CliRunner) -> None:
    """Test version command."""
    result = runner.invoke(cli, ["version"])
    assert result.exit_code == 0
    assert result.output.strip() != ""


def test_serve_command(runner: CliRunner) -> None:
    """Test serve command."""
    # Test that the command exists and shows help
    result = runner.invoke(cli, ["serve", "--help"])
    assert result.exit_code == 0
    assert "Start the kodit server" in result.output
