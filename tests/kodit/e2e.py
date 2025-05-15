"""End-to-end tests."""

import os
import shutil
import tempfile
from pathlib import Path
from typing import Generator

import pytest
from click.testing import CliRunner

from kodit.cli import cli
from kodit.config import reset_config, get_config


@pytest.fixture
def tmp_data_dir() -> Generator[Path, None, None]:
    """Create a temporary data directory."""
    with tempfile.TemporaryDirectory() as tmp_dir:
        yield Path(tmp_dir)


@pytest.fixture
def runner(tmp_data_dir: Path) -> Generator[CliRunner, None, None]:
    """Create a CliRunner instance."""
    reset_config()
    runner = CliRunner()
    runner.env = {
        "DATA_DIR": str(tmp_data_dir),
        "DB_URL": f"sqlite+aiosqlite:///{tmp_data_dir}/test.db",
        "DISABLE_TELEMETRY": "true",
    }
    yield runner


@pytest.fixture
def tmp_repo_dir() -> Generator[Path, None, None]:
    """Create a temporary test repository with some sample code."""
    with tempfile.TemporaryDirectory() as tmp_dir:
        yield Path(tmp_dir)


@pytest.fixture
def test_repo(tmp_repo_dir: Path) -> Generator[Path, None, None]:
    """Create a temporary test repository with some sample code."""
    # Create a sample Python file
    sample_file = tmp_repo_dir / "sample.py"
    sample_file.write_text("""
def hello_world():
    \"\"\"A simple hello world function.\"\"\"
    return "Hello, World!"

def add_numbers(a: int, b: int) -> int:
    \"\"\"Add two numbers together.\"\"\"
    return a + b
""")

    # Create a sample README
    readme = tmp_repo_dir / "README.md"
    readme.write_text(
        "# Test Repository\n\nThis is a test repository for kodit e2e tests."
    )

    yield tmp_repo_dir


def test_source_management(runner: CliRunner, test_repo: Path) -> None:
    """Test source management commands."""
    # Test creating a source
    result = runner.invoke(cli, ["sources", "create", str(test_repo)])
    assert result.exception is None
    assert result.exit_code == 0
    assert "Source created:" in result.output

    # Test listing sources
    result = runner.invoke(cli, ["sources", "list"])
    assert result.exit_code == 0
    assert str(test_repo) in result.output


def test_index_management(runner: CliRunner, test_repo: Path) -> None:
    """Test index management commands."""
    # Create a source first
    runner.invoke(cli, ["sources", "create", str(test_repo)])
    result = runner.invoke(cli, ["sources", "list"])
    print(result.output)

    # Test creating an index
    result = runner.invoke(cli, ["indexes", "create", "1"])
    print(result.exception)
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


def test_retrieval(runner: CliRunner, test_repo: Path) -> None:
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


def test_ensure_data_dir_exists(runner: CliRunner, tmp_data_dir: Path) -> None:
    """Ensure the data directory exists."""
    reset_config()
    subdir = tmp_data_dir / "test"
    # intentionally not creating the subdir
    runner.env = {
        "DATA_DIR": str(subdir),
        "DB_URL": f"sqlite+aiosqlite:///{subdir}/test.db",
        "DISABLE_TELEMETRY": "true",
    }
    result = runner.invoke(cli, ["sources", "list"])
    assert result.exit_code == 0
