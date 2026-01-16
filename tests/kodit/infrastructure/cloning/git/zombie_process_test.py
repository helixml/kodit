"""Test for GitPython zombie process leak on clone failure (Linux only)."""

import contextlib
import subprocess
import tempfile
from pathlib import Path

import pytest

from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter

FAILING_URL = "https://api:invalid@app.helix.ml/git/nonexistent"


def count_git_zombies() -> int:
    """Count zombie git processes via ps aux."""
    result = subprocess.run(  # noqa: S603
        ["/bin/ps", "aux"], capture_output=True, text=True, check=False
    )
    return sum(
        1
        for line in result.stdout.split("\n")
        if "defunct" in line and "git" in line.lower()
    )


@pytest.mark.asyncio
async def test_failed_clone_does_not_leave_zombie_processes() -> None:
    """Cloning a repo that fails should not leave zombie git processes."""
    adapter = GitPythonAdapter(max_workers=1)
    baseline = count_git_zombies()

    with tempfile.TemporaryDirectory() as tmpdir:
        for i in range(3):
            with contextlib.suppress(Exception):
                await adapter.clone_repository(FAILING_URL, Path(tmpdir) / f"clone-{i}")

    new_zombies = count_git_zombies() - baseline
    assert new_zombies == 0, (
        f"Clone failures created {new_zombies} zombie git process(es)"
    )


@pytest.mark.asyncio
async def test_dulwich_failed_clone_does_not_leave_zombie_processes() -> None:
    """Cloning a repo that fails with Dulwich should not leave zombie processes."""
    from kodit.infrastructure.cloning.git.dulwich_adaptor import DulwichAdapter

    adapter = DulwichAdapter(max_workers=1)
    baseline = count_git_zombies()

    with tempfile.TemporaryDirectory() as tmpdir:
        for i in range(3):
            with contextlib.suppress(Exception):
                await adapter.clone_repository(FAILING_URL, Path(tmpdir) / f"clone-{i}")

    new_zombies = count_git_zombies() - baseline
    assert new_zombies == 0, (
        f"Dulwich clone failures created {new_zombies} zombie git process(es)"
    )


@pytest.mark.asyncio
async def test_pygit2_failed_clone_does_not_leave_zombie_processes() -> None:
    """Cloning a repo that fails with PyGit2 should not leave zombie processes."""
    from kodit.infrastructure.cloning.git.pygit2_adaptor import PyGit2Adapter

    adapter = PyGit2Adapter(max_workers=1)
    baseline = count_git_zombies()

    with tempfile.TemporaryDirectory() as tmpdir:
        for i in range(3):
            with contextlib.suppress(Exception):
                await adapter.clone_repository(FAILING_URL, Path(tmpdir) / f"clone-{i}")

    new_zombies = count_git_zombies() - baseline
    assert new_zombies == 0, (
        f"PyGit2 clone failures created {new_zombies} zombie git process(es)"
    )
