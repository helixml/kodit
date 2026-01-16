"""Test if Dulwich clone produces a dirty working tree."""

import subprocess
import tempfile
from pathlib import Path

import pytest

from kodit.infrastructure.cloning.git.dulwich_adaptor import DulwichAdapter

# A public repo with .gitattributes that might cause issues
TEST_REPO = "https://github.com/helixml/kodit.git"


def get_git_status(repo: Path) -> str:
    """Get git status output."""
    result = subprocess.run(  # noqa: S603
        ["/usr/bin/git", "status", "--porcelain"],
        cwd=repo,
        capture_output=True,
        text=True,
        check=False,
    )
    return result.stdout.strip()


def get_modified_files(repo: Path) -> list[str]:
    """Get list of files that git sees as modified."""
    status = get_git_status(repo)
    if not status:
        return []
    return [line[3:] for line in status.split("\n") if line.startswith(" M")]


@pytest.mark.asyncio
async def test_dulwich_clone_produces_clean_worktree() -> None:
    """After Dulwich clone, git should see no modified files."""
    adapter = DulwichAdapter()

    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "repo"
        await adapter.clone_repository(TEST_REPO, repo_path)

        modified = get_modified_files(repo_path)
        assert modified == [], (
            f"Dulwich clone produced dirty worktree with modified files: {modified}"
        )


@pytest.mark.asyncio
async def test_dulwich_fetch_then_checkout_produces_clean_worktree() -> None:
    """After clone → fetch → checkout, git should see no modified files."""
    adapter = DulwichAdapter()

    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "repo"
        await adapter.clone_repository(TEST_REPO, repo_path)

        # Get current branch
        branch = await adapter.get_default_branch(repo_path)

        # This is the flow from git_repository_service.py
        await adapter.fetch_repository(repo_path)

        modified_after_fetch = get_modified_files(repo_path)
        assert modified_after_fetch == [], (
            f"Dulwich fetch produced dirty worktree: {modified_after_fetch}"
        )

        await adapter.checkout_branch(repo_path, branch)

        modified_after_checkout = get_modified_files(repo_path)
        assert modified_after_checkout == [], (
            f"Dulwich checkout produced dirty worktree: {modified_after_checkout}"
        )
