"""Test the CLI."""

import pytest
from click.testing import CliRunner

from kodit.cli import cli


@pytest.fixture
def runner() -> CliRunner:
    """Create a CliRunner instance."""
    return CliRunner()


def test_version_command(runner: CliRunner) -> None:
    """Test that the version command runs successfully."""
    result = runner.invoke(cli, ["version"])
    # The command should exit with success
    assert result.exit_code == 0
