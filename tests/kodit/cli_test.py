"""Test the CLI."""

import tempfile
from typing import Generator
import pytest
from click.testing import CliRunner

from kodit.cli import cli
from kodit.config import reset_config


@pytest.fixture
def runner() -> Generator[CliRunner, None, None]:
    """Create a CliRunner instance."""
    reset_config()
    yield CliRunner()


def test_version_command(runner: CliRunner) -> None:
    """Test that the version command runs successfully."""
    result = runner.invoke(cli, ["version"])
    # The command should exit with success
    assert result.exit_code == 0


def test_cli_vars_work(runner: CliRunner) -> None:
    """Test that cli args override env vars."""
    runner.env = {"LOG_LEVEL": "INFO"}
    result = runner.invoke(cli, ["--log-level", "DEBUG", "sources", "list"])
    assert result.exit_code == 0
    assert result.output.count("debug") > 10  # The db spits out lots of debug messages


def test_env_vars_work(runner: CliRunner) -> None:
    """Test that env vars work."""
    runner.env = {"LOG_LEVEL": "DEBUG"}
    result = runner.invoke(cli, ["sources", "list"])
    assert result.exit_code == 0
    assert result.output.count("debug") > 10  # The db spits out lots of debug messages


def test_dotenv_file_works(runner: CliRunner) -> None:
    """Test that the .env file works."""
    with tempfile.NamedTemporaryFile(delete=False) as f:
        f.write(b"LOG_LEVEL=DEBUG")
        f.flush()
        result = runner.invoke(cli, ["--env-file", f.name, "sources", "list"])
        assert result.exit_code == 0
        assert (
            result.output.count("debug") > 10
        )  # The db spits out lots of debug messages
