"""Test that DulwichAdapter handles git submodules correctly."""

import subprocess
import tempfile
from pathlib import Path

import pytest

from kodit.infrastructure.cloning.git.dulwich_adaptor import DulwichAdapter


def git(repo: Path, *args: str) -> str:
    """Run a git command in the given repo."""
    result = subprocess.run(  # noqa: S603
        ["git", *args], cwd=repo, check=True, capture_output=True, text=True  # noqa: S607
    )
    return result.stdout.strip()


@pytest.mark.asyncio
async def test_get_commit_files_skips_submodules() -> None:
    """Submodule entries (gitlinks) are skipped without raising KeyError."""
    with tempfile.TemporaryDirectory() as tmpdir:
        tmppath = Path(tmpdir)
        external_repo = tmppath / "external"
        main_repo = tmppath / "main"

        # Create an external repo to get a real commit SHA
        external_repo.mkdir()
        git(external_repo, "init", "-b", "main")
        git(external_repo, "config", "user.email", "test@example.com")
        git(external_repo, "config", "user.name", "Test User")
        (external_repo / "lib.py").write_text("# external lib")
        git(external_repo, "add", "lib.py")
        git(external_repo, "commit", "-m", "External commit")
        external_sha = git(external_repo, "rev-parse", "HEAD")

        # Create main repository
        main_repo.mkdir()
        git(main_repo, "init", "-b", "main")
        git(main_repo, "config", "user.email", "test@example.com")
        git(main_repo, "config", "user.name", "Test User")

        # Add a regular file
        (main_repo / "main.py").write_text("# main code")
        git(main_repo, "add", "main.py")

        # Add a gitlink entry (mode 160000) pointing to the external repo's commit.
        # This simulates a submodule without needing git submodule add.
        git(
            main_repo,
            "update-index",
            "--add",
            "--cacheinfo",
            f"160000,{external_sha},vendor/lib",
        )

        git(main_repo, "commit", "-m", "Add file and submodule reference")

        commit_sha = git(main_repo, "rev-parse", "HEAD")

        # Get files - should not raise KeyError for the gitlink
        adapter = DulwichAdapter()
        files = await adapter.get_commit_files(main_repo, commit_sha)

        paths = [f["path"] for f in files]

        # Should include regular files
        assert "main.py" in paths

        # Should NOT include the submodule path (gitlink with mode 160000)
        assert "vendor/lib" not in paths
