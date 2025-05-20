"""Test the CLI."""

import tempfile
from typing import Generator
import pytest
from click.testing import CliRunner

from kodit.cli import cli
from kodit.config import AppContext


@pytest.fixture
def runner() -> Generator[CliRunner, None, None]:
    """Create a CliRunner instance."""
    yield CliRunner()


@pytest.fixture
def default_cli_args(app_context: AppContext) -> list[str]:
    """Get the default CLI args."""
    return [
        "--disable-telemetry",
        "--data-dir",
        str(app_context.get_data_dir()),
        "--db-url",
        app_context.db_url,
    ]


def test_version_command(runner: CliRunner, default_cli_args: list[str]) -> None:
    """Test that the version command runs successfully."""
    result = runner.invoke(cli, [*default_cli_args, "version"])
    # The command should exit with success
    assert result.exit_code == 0


def test_cli_vars_work(runner: CliRunner, default_cli_args: list[str]) -> None:
    """Test that cli args override env vars."""
    runner.env = {"LOG_LEVEL": "INFO"}
    result = runner.invoke(cli, [*default_cli_args, "--log-level", "DEBUG", "index"])
    assert result.exit_code == 0
    assert result.output.count("debug") > 10  # The db spits out lots of debug messages


def test_env_vars_work(runner: CliRunner, default_cli_args: list[str]) -> None:
    """Test that env vars work."""
    runner.env = {"LOG_LEVEL": "DEBUG"}
    result = runner.invoke(cli, [*default_cli_args, "index"])
    assert result.exit_code == 0
    assert result.output.count("debug") > 10  # The db spits out lots of debug messages


def test_dotenv_file_works(runner: CliRunner, default_cli_args: list[str]) -> None:
    """Test that the .env file works."""
    with tempfile.NamedTemporaryFile(delete=False) as f:
        f.write(b"LOG_LEVEL=DEBUG")
        f.flush()
        result = runner.invoke(cli, [*default_cli_args, "--env-file", f.name, "index"])
        assert result.exit_code == 0
        assert (
            result.output.count("debug") > 10
        )  # The db spits out lots of debug messages


def test_dotenv_file_not_found(runner: CliRunner, default_cli_args: list[str]) -> None:
    """Test that the .env file not found error is raised."""
    result = runner.invoke(
        cli, [*default_cli_args, "--env-file", "nonexistent.env", "index"]
    )
    assert result.exit_code == 2
    assert "does not exist" in result.output
