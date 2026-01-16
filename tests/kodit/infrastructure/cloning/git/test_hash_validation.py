"""Test that all Git adapters return valid 40-character hex hashes."""

import re
import subprocess
import tempfile
from collections.abc import Generator
from pathlib import Path

import pytest

from kodit.infrastructure.cloning.git.dulwich_adaptor import DulwichAdapter
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.cloning.git.pygit2_adaptor import PyGit2Adapter

VALID_SHA_PATTERN = re.compile(r"^[0-9a-f]{40}$")


def is_valid_sha(value: str) -> bool:
    """Check if a string is a valid 40-character hex SHA."""
    return bool(VALID_SHA_PATTERN.match(value))


@pytest.fixture
def temp_repo() -> Generator[tuple[Path, str], None, None]:
    """Create a temporary git repository with one commit."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir)

        # Initialize repo with explicit branch name
        subprocess.run(  # noqa: S603
            ["git", "init", "-b", "main"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )

        # Configure git user
        subprocess.run(  # noqa: S603
            ["git", "config", "user.email", "test@example.com"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(  # noqa: S603
            ["git", "config", "user.name", "Test User"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )

        # Create a file
        test_file = repo_path / "test.txt"
        test_file.write_text("Hello, World!\n")

        # Add and commit
        subprocess.run(  # noqa: S603
            ["git", "add", "test.txt"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(  # noqa: S603
            ["git", "commit", "-m", "Initial commit"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )

        yield repo_path, "main"


@pytest.mark.asyncio
async def test_gitpython_adapter_returns_valid_hashes(
    temp_repo: tuple[Path, str],
) -> None:
    """Test GitPythonAdapter returns valid SHA hashes."""
    repo_path, branch_name = temp_repo
    adapter = GitPythonAdapter()

    # Test get_latest_commit_sha
    commit_sha = await adapter.get_latest_commit_sha(repo_path)
    assert is_valid_sha(commit_sha), f"Invalid commit SHA: {commit_sha}"

    # Test get_commit_details
    details = await adapter.get_commit_details(repo_path, commit_sha)
    assert is_valid_sha(details["sha"]), f"Invalid sha in details: {details['sha']}"
    assert is_valid_sha(
        details["tree_sha"]
    ), f"Invalid tree_sha: {details['tree_sha']}"

    # Test get_commit_files
    files = await adapter.get_commit_files(repo_path, commit_sha)
    assert len(files) == 1
    assert is_valid_sha(
        files[0]["blob_sha"]
    ), f"Invalid blob_sha: {files[0]['blob_sha']}"

    # Test get_branch_commits
    commits = await adapter.get_branch_commits(repo_path, branch_name)
    assert len(commits) == 1
    assert is_valid_sha(commits[0]["sha"]), f"Invalid sha: {commits[0]['sha']}"
    assert is_valid_sha(
        commits[0]["tree_sha"]
    ), f"Invalid tree_sha: {commits[0]['tree_sha']}"


@pytest.mark.asyncio
async def test_pygit2_adapter_returns_valid_hashes(
    temp_repo: tuple[Path, str],
) -> None:
    """Test PyGit2Adapter returns valid SHA hashes."""
    repo_path, branch_name = temp_repo
    adapter = PyGit2Adapter()

    # Test get_latest_commit_sha
    commit_sha = await adapter.get_latest_commit_sha(repo_path)
    assert is_valid_sha(commit_sha), f"Invalid commit SHA: {commit_sha}"

    # Test get_commit_details
    details = await adapter.get_commit_details(repo_path, commit_sha)
    assert is_valid_sha(details["sha"]), f"Invalid sha in details: {details['sha']}"
    assert is_valid_sha(
        details["tree_sha"]
    ), f"Invalid tree_sha: {details['tree_sha']}"

    # Test get_commit_files
    files = await adapter.get_commit_files(repo_path, commit_sha)
    assert len(files) == 1
    assert is_valid_sha(
        files[0]["blob_sha"]
    ), f"Invalid blob_sha: {files[0]['blob_sha']}"

    # Test get_branch_commits
    commits = await adapter.get_branch_commits(repo_path, branch_name)
    assert len(commits) == 1
    assert is_valid_sha(commits[0]["sha"]), f"Invalid sha: {commits[0]['sha']}"
    assert is_valid_sha(
        commits[0]["tree_sha"]
    ), f"Invalid tree_sha: {commits[0]['tree_sha']}"


@pytest.mark.asyncio
async def test_dulwich_adapter_returns_valid_hashes(
    temp_repo: tuple[Path, str],
) -> None:
    """Test DulwichAdapter returns valid SHA hashes."""
    repo_path, branch_name = temp_repo
    adapter = DulwichAdapter()

    # Test get_latest_commit_sha
    commit_sha = await adapter.get_latest_commit_sha(repo_path)
    assert is_valid_sha(commit_sha), f"Invalid commit SHA: {commit_sha}"

    # Test get_commit_details
    details = await adapter.get_commit_details(repo_path, commit_sha)
    assert is_valid_sha(details["sha"]), f"Invalid sha in details: {details['sha']}"
    assert is_valid_sha(
        details["tree_sha"]
    ), f"Invalid tree_sha: {details['tree_sha']}"

    # Test get_commit_files
    files = await adapter.get_commit_files(repo_path, commit_sha)
    assert len(files) == 1
    assert is_valid_sha(
        files[0]["blob_sha"]
    ), f"Invalid blob_sha: {files[0]['blob_sha']}"

    # Test get_branch_commits
    commits = await adapter.get_branch_commits(repo_path, branch_name)
    assert len(commits) == 1
    assert is_valid_sha(commits[0]["sha"]), f"Invalid sha: {commits[0]['sha']}"
    assert is_valid_sha(
        commits[0]["tree_sha"]
    ), f"Invalid tree_sha: {commits[0]['tree_sha']}"


@pytest.mark.asyncio
async def test_all_adapters_return_same_hashes(temp_repo: tuple[Path, str]) -> None:
    """Test all adapters return the same hash values for the same repo."""
    repo_path, _ = temp_repo
    gitpython = GitPythonAdapter()
    pygit2 = PyGit2Adapter()
    dulwich = DulwichAdapter()

    # Get commit SHA from all adapters
    sha_gitpython = await gitpython.get_latest_commit_sha(repo_path)
    sha_pygit2 = await pygit2.get_latest_commit_sha(repo_path)
    sha_dulwich = await dulwich.get_latest_commit_sha(repo_path)

    assert sha_gitpython == sha_pygit2, "GitPython and PyGit2 SHAs don't match"
    assert sha_gitpython == sha_dulwich, "GitPython and Dulwich SHAs don't match"

    # Get commit details from all adapters
    details_gitpython = await gitpython.get_commit_details(repo_path, sha_gitpython)
    details_pygit2 = await pygit2.get_commit_details(repo_path, sha_pygit2)
    details_dulwich = await dulwich.get_commit_details(repo_path, sha_dulwich)

    # Compare tree SHAs
    assert (
        details_gitpython["tree_sha"] == details_pygit2["tree_sha"]
    ), "Tree SHAs don't match between GitPython and PyGit2"
    assert (
        details_gitpython["tree_sha"] == details_dulwich["tree_sha"]
    ), "Tree SHAs don't match between GitPython and Dulwich"

    # Get files from all adapters
    files_gitpython = await gitpython.get_commit_files(repo_path, sha_gitpython)
    files_pygit2 = await pygit2.get_commit_files(repo_path, sha_pygit2)
    files_dulwich = await dulwich.get_commit_files(repo_path, sha_dulwich)

    # Compare blob SHAs
    assert (
        files_gitpython[0]["blob_sha"] == files_pygit2[0]["blob_sha"]
    ), "Blob SHAs don't match between GitPython and PyGit2"
    assert (
        files_gitpython[0]["blob_sha"] == files_dulwich[0]["blob_sha"]
    ), "Blob SHAs don't match between GitPython and Dulwich"


# Additional tests for other Dulwich adapter methods that use .hex()


@pytest.mark.asyncio
async def test_dulwich_get_all_branches_returns_valid_hashes(
    temp_repo: tuple[Path, str],
) -> None:
    """Test DulwichAdapter.get_all_branches returns valid head_commit_sha."""
    repo_path, _ = temp_repo
    adapter = DulwichAdapter()

    branches = await adapter.get_all_branches(repo_path)
    assert len(branches) >= 1

    for branch in branches:
        sha = branch["head_commit_sha"]
        assert is_valid_sha(sha), f"Invalid head_commit_sha in branch: {sha}"


@pytest.mark.asyncio
async def test_dulwich_get_all_branch_head_shas_returns_valid_hashes(
    temp_repo: tuple[Path, str],
) -> None:
    """Test DulwichAdapter.get_all_branch_head_shas returns valid SHAs."""
    repo_path, branch_name = temp_repo
    adapter = DulwichAdapter()

    head_shas = await adapter.get_all_branch_head_shas(repo_path, [branch_name])
    assert branch_name in head_shas

    sha = head_shas[branch_name]
    assert is_valid_sha(sha), f"Invalid SHA from get_all_branch_head_shas: {sha}"


@pytest.mark.asyncio
async def test_dulwich_get_branch_commit_shas_returns_valid_hashes(
    temp_repo: tuple[Path, str],
) -> None:
    """Test DulwichAdapter.get_branch_commit_shas returns valid SHAs."""
    repo_path, branch_name = temp_repo
    adapter = DulwichAdapter()

    shas = await adapter.get_branch_commit_shas(repo_path, branch_name)
    assert len(shas) >= 1

    for sha in shas:
        assert is_valid_sha(sha), f"Invalid SHA from get_branch_commit_shas: {sha}"


@pytest.mark.asyncio
async def test_dulwich_get_all_commits_bulk_returns_valid_hashes(
    temp_repo: tuple[Path, str],
) -> None:
    """Test DulwichAdapter.get_all_commits_bulk returns valid SHAs."""
    repo_path, _ = temp_repo
    adapter = DulwichAdapter()

    commits = await adapter.get_all_commits_bulk(repo_path)
    assert len(commits) >= 1

    for sha, commit_data in commits.items():
        assert is_valid_sha(sha), f"Invalid key SHA in commits dict: {sha}"
        assert is_valid_sha(
            commit_data["sha"]
        ), f"Invalid sha in commit data: {commit_data['sha']}"
        assert is_valid_sha(
            commit_data["tree_sha"]
        ), f"Invalid tree_sha in commit data: {commit_data['tree_sha']}"


@pytest.fixture
def temp_repo_with_parent() -> Generator[tuple[Path, str], None, None]:
    """Create a temporary git repository with two commits (to test parent_sha)."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir)

        subprocess.run(  # noqa: S603
            ["git", "init", "-b", "main"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(  # noqa: S603
            ["git", "config", "user.email", "test@example.com"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(  # noqa: S603
            ["git", "config", "user.name", "Test User"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )

        # First commit
        test_file = repo_path / "test.txt"
        test_file.write_text("First version\n")
        subprocess.run(  # noqa: S603
            ["git", "add", "test.txt"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(  # noqa: S603
            ["git", "commit", "-m", "First commit"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )

        # Second commit
        test_file.write_text("Second version\n")
        subprocess.run(  # noqa: S603
            ["git", "add", "test.txt"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(  # noqa: S603
            ["git", "commit", "-m", "Second commit"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )

        yield repo_path, "main"


@pytest.mark.asyncio
async def test_dulwich_parent_sha_is_valid(
    temp_repo_with_parent: tuple[Path, str],
) -> None:
    """Test DulwichAdapter returns valid parent_sha for commits with parents."""
    repo_path, branch_name = temp_repo_with_parent
    adapter = DulwichAdapter()

    commit_sha = await adapter.get_latest_commit_sha(repo_path)
    details = await adapter.get_commit_details(repo_path, commit_sha)

    # The second commit should have a parent
    parent_sha = details["parent_sha"]
    assert parent_sha, "Expected a parent_sha for the second commit"
    assert is_valid_sha(parent_sha), f"Invalid parent_sha: {parent_sha}"


@pytest.fixture
def temp_repo_with_tag() -> Generator[tuple[Path, str], None, None]:
    """Create a temporary git repository with a tag."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir)

        subprocess.run(  # noqa: S603
            ["git", "init", "-b", "main"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(  # noqa: S603
            ["git", "config", "user.email", "test@example.com"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(  # noqa: S603
            ["git", "config", "user.name", "Test User"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )

        test_file = repo_path / "test.txt"
        test_file.write_text("Hello\n")
        subprocess.run(  # noqa: S603
            ["git", "add", "test.txt"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(  # noqa: S603
            ["git", "commit", "-m", "Initial commit"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )

        # Create a lightweight tag
        subprocess.run(  # noqa: S603
            ["git", "tag", "v1.0"],
            cwd=repo_path,
            check=True,
            capture_output=True,
        )

        yield repo_path, "main"


@pytest.mark.asyncio
async def test_dulwich_get_all_tags_returns_valid_hashes(
    temp_repo_with_tag: tuple[Path, str],
) -> None:
    """Test DulwichAdapter.get_all_tags returns valid commit SHAs."""
    repo_path, _ = temp_repo_with_tag
    adapter = DulwichAdapter()

    tags = await adapter.get_all_tags(repo_path)
    assert len(tags) >= 1

    for tag in tags:
        sha = tag["target_commit_sha"]
        assert is_valid_sha(sha), f"Invalid target_commit_sha in tag: {sha}"


# Tests for Dulwich input handling (bytes.fromhex bug)


@pytest.mark.asyncio
async def test_dulwich_get_commit_files_with_valid_sha(
    temp_repo: tuple[Path, str],
) -> None:
    """Test DulwichAdapter.get_commit_files works with a valid 40-char SHA."""
    repo_path, _ = temp_repo

    # Get a known-good SHA from GitPython
    gitpython = GitPythonAdapter()
    valid_sha = await gitpython.get_latest_commit_sha(repo_path)
    assert is_valid_sha(valid_sha), "GitPython should return valid SHA"

    # Try to use it with Dulwich
    dulwich = DulwichAdapter()
    files = await dulwich.get_commit_files(repo_path, valid_sha)
    assert len(files) >= 1, "Should return at least one file"


@pytest.mark.asyncio
async def test_dulwich_get_commit_details_with_valid_sha(
    temp_repo: tuple[Path, str],
) -> None:
    """Test DulwichAdapter.get_commit_details works with a valid 40-char SHA."""
    repo_path, _ = temp_repo

    # Get a known-good SHA from GitPython
    gitpython = GitPythonAdapter()
    valid_sha = await gitpython.get_latest_commit_sha(repo_path)
    assert is_valid_sha(valid_sha), "GitPython should return valid SHA"

    # Try to use it with Dulwich
    dulwich = DulwichAdapter()
    details = await dulwich.get_commit_details(repo_path, valid_sha)
    assert "sha" in details, "Should return commit details"


@pytest.mark.asyncio
async def test_dulwich_get_commit_diff_with_valid_sha(
    temp_repo_with_parent: tuple[Path, str],
) -> None:
    """Test DulwichAdapter.get_commit_diff works with a valid 40-char SHA."""
    repo_path, _ = temp_repo_with_parent

    # Get a known-good SHA from GitPython
    gitpython = GitPythonAdapter()
    valid_sha = await gitpython.get_latest_commit_sha(repo_path)
    assert is_valid_sha(valid_sha), "GitPython should return valid SHA"

    # Try to use it with Dulwich
    dulwich = DulwichAdapter()
    diff = await dulwich.get_commit_diff(repo_path, valid_sha)
    # Just check it returns a string without throwing
    assert isinstance(diff, str), "Should return diff string"
