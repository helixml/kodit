"""Test that Dulwich force checkout works with uncommitted changes."""

import subprocess
import tempfile
from pathlib import Path

import pytest

from kodit.infrastructure.cloning.git.dulwich_adaptor import DulwichAdapter


def git(repo: Path, *args: str) -> None:
    """Run a git command in the given repo."""
    subprocess.run(["git", *args], cwd=repo, check=True, capture_output=True)  # noqa: S603, S607


@pytest.mark.asyncio
async def test_checkout_with_uncommitted_changes_succeeds_with_force() -> None:
    """Checkout succeeds even with uncommitted local changes (force=True)."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo = Path(tmpdir)

        # Initialize repo with a commit
        git(repo, "init", "-b", "main")
        git(repo, "config", "user.email", "test@example.com")
        git(repo, "config", "user.name", "Test User")
        (repo / "file.txt").write_text("initial")
        git(repo, "add", "file.txt")
        git(repo, "commit", "-m", "Initial commit")

        # Create another branch
        git(repo, "branch", "other")

        # Modify file without committing
        (repo / "file.txt").write_text("modified")

        # Checkout should succeed with force=True
        adapter = DulwichAdapter()
        await adapter.checkout_branch(repo, "other")

        # File should be reverted to committed state
        assert (repo / "file.txt").read_text() == "initial"
