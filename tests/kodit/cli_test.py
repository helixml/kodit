"""Test the CLI."""

from pathlib import Path
import tempfile
from typing import Generator
import pytest
from click.testing import CliRunner

from kodit.cli import cli


@pytest.fixture
def tmp_data_dir() -> Generator[Path, None, None]:
    """Create a temporary data directory."""
    with tempfile.TemporaryDirectory() as tmp_dir:
        yield Path(tmp_dir)


@pytest.fixture
def runner(tmp_data_dir: Path) -> Generator[CliRunner, None, None]:
    """Create a CliRunner instance."""
    runner = CliRunner()
    runner.env = {
        "DISABLE_TELEMETRY": "true",
        "DATA_DIR": str(tmp_data_dir),
        "DB_URL": f"sqlite+aiosqlite:///{tmp_data_dir}/test.db",
    }
    yield runner


def test_version_command(runner: CliRunner) -> None:
    """Test that the version command runs successfully."""
    result = runner.invoke(cli, ["version"])
    # The command should exit with success
    assert result.exit_code == 0


def test_telemetry_disabled_in_these_tests(runner: CliRunner) -> None:
    """Test that telemetry is disabled in these tests."""
    result = runner.invoke(cli, ["version"])
    assert result.exit_code == 0
    assert "Telemetry has been disabled" in result.output


def test_env_vars_work(runner: CliRunner) -> None:
    """Test that env vars work."""
    runner.env = {**runner.env, "LOG_LEVEL": "DEBUG"}
    result = runner.invoke(cli, ["index"])
    assert result.exit_code == 0
    assert result.output.count("debug") > 10  # The db spits out lots of debug messages


def test_dotenv_file_works(runner: CliRunner) -> None:
    """Test that the .env file works."""
    with tempfile.NamedTemporaryFile(delete=False) as f:
        f.write(b"LOG_LEVEL=DEBUG")
        f.flush()
        result = runner.invoke(cli, ["--env-file", f.name, "index"])
        assert result.exit_code == 0
        assert (
            result.output.count("debug") > 10
        )  # The db spits out lots of debug messages


def test_dotenv_file_not_found(runner: CliRunner) -> None:
    """Test that the .env file not found error is raised."""
    result = runner.invoke(cli, ["--env-file", "nonexistent.env", "index"])
    assert result.exit_code == 2
    assert "does not exist" in result.output
