"""Test that all Git adapters return valid 40-character hex hashes."""

import re
import subprocess
import tempfile
from collections.abc import Generator
from pathlib import Path
from typing import Any

import pytest

from kodit.domain.protocols import GitAdapter
from kodit.infrastructure.cloning.git.dulwich_adaptor import DulwichAdapter
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.cloning.git.pygit2_adaptor import PyGit2Adapter

VALID_SHA = re.compile(r"^[0-9a-f]{40}$")
ADAPTERS: list[type[GitAdapter]] = [GitPythonAdapter, PyGit2Adapter, DulwichAdapter]


def is_valid_sha(value: str) -> bool:
    """Check if a string is a valid 40-character hex SHA."""
    return bool(VALID_SHA.match(value))


def git(repo: Path, *args: str) -> None:
    """Run a git command in the given repo."""
    subprocess.run(["git", *args], cwd=repo, check=True, capture_output=True)  # noqa: S603, S607


def init_repo(repo: Path) -> None:
    """Initialize a git repo with user config."""
    git(repo, "init", "-b", "main")
    git(repo, "config", "user.email", "test@example.com")
    git(repo, "config", "user.name", "Test User")


def commit_file(repo: Path, filename: str, content: str, message: str) -> None:
    """Create/update a file and commit it."""
    (repo / filename).write_text(content)
    git(repo, "add", filename)
    git(repo, "commit", "-m", message)


@pytest.fixture
def temp_repo() -> Generator[tuple[Path, str], None, None]:
    """Create a temporary git repository with one commit."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo = Path(tmpdir)
        init_repo(repo)
        commit_file(repo, "test.txt", "Hello\n", "Initial commit")
        yield repo, "main"


@pytest.fixture
def temp_repo_with_parent() -> Generator[tuple[Path, str], None, None]:
    """Create a temporary git repository with two commits."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo = Path(tmpdir)
        init_repo(repo)
        commit_file(repo, "test.txt", "First\n", "First commit")
        commit_file(repo, "test.txt", "Second\n", "Second commit")
        yield repo, "main"


@pytest.fixture
def temp_repo_with_tag() -> Generator[tuple[Path, str], None, None]:
    """Create a temporary git repository with a tag."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo = Path(tmpdir)
        init_repo(repo)
        commit_file(repo, "test.txt", "Hello\n", "Initial commit")
        git(repo, "tag", "v1.0")
        yield repo, "main"


@pytest.fixture(params=ADAPTERS, ids=lambda c: c.__name__)
def adapter(request: Any) -> GitAdapter:
    """Parametrized fixture for all adapter types."""
    return request.param()


@pytest.mark.asyncio
async def test_adapter_returns_valid_hashes(
    temp_repo: tuple[Path, str], adapter: GitAdapter
) -> None:
    """Test adapter returns valid SHA hashes for all hash-returning methods."""
    repo, branch = temp_repo

    sha = await adapter.get_latest_commit_sha(repo)
    assert is_valid_sha(sha), f"Invalid commit SHA: {sha}"

    details = await adapter.get_commit_details(repo, sha)
    assert is_valid_sha(details["sha"])
    assert is_valid_sha(details["tree_sha"])

    files = await adapter.get_commit_files(repo, sha)
    assert is_valid_sha(files[0]["blob_sha"])

    commits = await adapter.get_branch_commits(repo, branch)
    assert is_valid_sha(commits[0]["sha"])
    assert is_valid_sha(commits[0]["tree_sha"])


@pytest.mark.asyncio
async def test_all_adapters_return_same_hashes(temp_repo: tuple[Path, str]) -> None:
    """Test all adapters return identical hash values for the same repo."""
    repo, _ = temp_repo
    adapters: list[GitAdapter] = [cls() for cls in ADAPTERS]

    shas = [await a.get_latest_commit_sha(repo) for a in adapters]
    assert len(set(shas)) == 1, f"Adapters returned different SHAs: {shas}"

    details = [await a.get_commit_details(repo, shas[0]) for a in adapters]
    tree_shas = [d["tree_sha"] for d in details]
    assert len(set(tree_shas)) == 1, f"Different tree SHAs: {tree_shas}"

    files = [await a.get_commit_files(repo, shas[0]) for a in adapters]
    blob_shas = [f[0]["blob_sha"] for f in files]
    assert len(set(blob_shas)) == 1, f"Different blob SHAs: {blob_shas}"


@pytest.mark.asyncio
async def test_dulwich_branch_methods_return_valid_hashes(
    temp_repo: tuple[Path, str],
) -> None:
    """Test Dulwich branch-related methods return valid hashes."""
    repo, branch = temp_repo
    adapter = DulwichAdapter()

    branches = await adapter.get_all_branches(repo)
    assert all(is_valid_sha(b["head_commit_sha"]) for b in branches)

    head_shas = await adapter.get_all_branch_head_shas(repo, [branch])
    assert is_valid_sha(head_shas[branch])

    commit_shas = await adapter.get_branch_commit_shas(repo, branch)
    assert all(is_valid_sha(sha) for sha in commit_shas)

    bulk = await adapter.get_all_commits_bulk(repo)
    for sha, data in bulk.items():
        assert is_valid_sha(sha)
        assert is_valid_sha(data["sha"])
        assert is_valid_sha(data["tree_sha"])


@pytest.mark.asyncio
async def test_dulwich_parent_sha_is_valid(
    temp_repo_with_parent: tuple[Path, str],
) -> None:
    """Test Dulwich returns valid parent_sha for commits with parents."""
    repo, _ = temp_repo_with_parent
    adapter = DulwichAdapter()

    sha = await adapter.get_latest_commit_sha(repo)
    details = await adapter.get_commit_details(repo, sha)
    assert is_valid_sha(details["parent_sha"])


@pytest.mark.asyncio
async def test_dulwich_tag_hashes_are_valid(
    temp_repo_with_tag: tuple[Path, str],
) -> None:
    """Test Dulwich returns valid commit SHAs for tags."""
    repo, _ = temp_repo_with_tag
    adapter = DulwichAdapter()

    tags = await adapter.get_all_tags(repo)
    assert all(is_valid_sha(t["target_commit_sha"]) for t in tags)


@pytest.mark.asyncio
async def test_dulwich_accepts_valid_sha_input(
    temp_repo_with_parent: tuple[Path, str],
) -> None:
    """Test Dulwich methods work with valid 40-char SHA inputs from other adapters."""
    repo, _ = temp_repo_with_parent
    sha = await GitPythonAdapter().get_latest_commit_sha(repo)
    dulwich = DulwichAdapter()

    files = await dulwich.get_commit_files(repo, sha)
    assert len(files) >= 1

    details = await dulwich.get_commit_details(repo, sha)
    assert "sha" in details

    diff = await dulwich.get_commit_diff(repo, sha)
    assert isinstance(diff, str)
