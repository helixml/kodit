"""Test the CLI."""

import tempfile
from typing import Generator
import pytest
from click.testing import CliRunner

from kodit.cli import cli


@pytest.fixture
def runner() -> Generator[CliRunner, None, None]:
    """Create a CliRunner instance."""
    runner = CliRunner()
    runner.env = {"DISABLE_TELEMETRY": "true"}
    yield runner


def test_version_command(runner: CliRunner) -> None:
    """Test that the version command runs successfully."""
    result = runner.invoke(cli, ["version"])
    # The command should exit with success
    assert result.exit_code == 0


def test_telemetry_disabled_by_default(runner: CliRunner) -> None:
    """Test that telemetry is disabled by default."""
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
