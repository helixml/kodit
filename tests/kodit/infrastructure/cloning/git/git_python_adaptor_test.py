"""Tests for GitPython adapter module."""

import asyncio
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from unittest.mock import MagicMock, patch

import pytest
from git import GitCommandError, InvalidGitRepositoryError

from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter

MODULE_PATH = "kodit.infrastructure.cloning.git.git_python_adaptor"


@pytest.fixture
def git_adapter() -> GitPythonAdapter:
    """Create a GitPythonAdapter instance."""
    return GitPythonAdapter(max_workers=2)


@pytest.fixture
def mock_repo() -> MagicMock:
    """Create a mock Git repository."""
    repo = MagicMock()
    repo.tags = []
    return repo


@pytest.fixture
def mock_commit() -> MagicMock:
    """Create a mock Git commit."""
    commit = MagicMock()
    commit.hexsha = "abc123def456"
    commit.committed_date = 1640995200  # 2022-01-01 00:00:00 UTC
    commit.message = "Test commit message"
    commit.parents = []
    commit.author.name = "Test Author"
    commit.author.email = "test@example.com"
    commit.committer.name = "Test Committer"
    commit.committer.email = "committer@example.com"
    commit.tree.hexsha = "tree123"
    commit.stats.total = {
        "insertions": 10,
        "deletions": 5,
        "lines": 15,
        "files": 2,
    }
    return commit


@pytest.mark.asyncio
async def test_clone_repository_success(git_adapter: GitPythonAdapter) -> None:
    """Test successful repository cloning."""
    remote_uri = "https://github.com/test/repo.git"
    local_path = Path("/tmp/test-repo")

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo = MagicMock()
        mock_repo.tags = ["v1.0", "v2.0"]
        mock_repo_class.clone_from.return_value = mock_repo

        await git_adapter.clone_repository(remote_uri, local_path)

        mock_repo_class.clone_from.assert_called_once_with(remote_uri, local_path)


@pytest.mark.asyncio
async def test_clone_repository_removes_existing_directory(
    git_adapter: GitPythonAdapter,
) -> None:
    """Test that existing directory is removed before cloning."""
    remote_uri = "https://github.com/test/repo.git"
    local_path = Path("/tmp/test-repo")

    with (
        patch(f"{MODULE_PATH}.Repo") as mock_repo_class,
        patch(f"{MODULE_PATH}.shutil.rmtree") as mock_rmtree,
        patch("pathlib.Path.exists", return_value=True),
        patch("pathlib.Path.mkdir"),
    ):
        mock_repo = MagicMock()
        mock_repo.tags = []
        mock_repo_class.clone_from.return_value = mock_repo

        await git_adapter.clone_repository(remote_uri, local_path)

        mock_rmtree.assert_called_once_with(local_path)
        mock_repo_class.clone_from.assert_called_once_with(remote_uri, local_path)


@pytest.mark.asyncio
async def test_clone_repository_failure(git_adapter: GitPythonAdapter) -> None:
    """Test handling of clone failures."""
    remote_uri = "https://github.com/test/nonexistent.git"
    local_path = Path("/tmp/test-repo")

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo_class.clone_from.side_effect = GitCommandError(
            "clone", "Repository not found"
        )

        with pytest.raises(GitCommandError):
            await git_adapter.clone_repository(remote_uri, local_path)


@pytest.mark.asyncio
async def test_pull_repository_success(git_adapter: GitPythonAdapter) -> None:
    """Test successful repository pull."""
    local_path = Path("/tmp/test-repo")

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo = MagicMock()
        mock_origin = MagicMock()
        mock_repo.remotes.origin = mock_origin
        mock_repo_class.return_value = mock_repo

        await git_adapter.pull_repository(local_path)

        mock_repo_class.assert_called_once_with(local_path)
        mock_origin.pull.assert_called_once()


@pytest.mark.asyncio
async def test_pull_repository_failure(git_adapter: GitPythonAdapter) -> None:
    """Test handling of pull failures."""
    local_path = Path("/tmp/test-repo")

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo_class.side_effect = InvalidGitRepositoryError("Not a git repository")

        with pytest.raises(InvalidGitRepositoryError):
            await git_adapter.pull_repository(local_path)


@pytest.mark.asyncio
async def test_get_all_branches_with_local_and_remote(
    git_adapter: GitPythonAdapter,
) -> None:
    """Test getting all branches including local and remote."""
    local_path = Path("/tmp/test-repo")

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        # Create mock branches
        mock_main_branch = MagicMock()
        mock_main_branch.name = "main"
        mock_main_branch.commit.hexsha = "abc123"

        mock_dev_branch = MagicMock()
        mock_dev_branch.name = "develop"
        mock_dev_branch.commit.hexsha = "def456"

        # Create mock repository
        mock_repo = MagicMock()
        mock_repo.active_branch = mock_main_branch
        mock_repo.branches = [mock_main_branch, mock_dev_branch]

        # Create mock remote
        mock_remote = MagicMock()
        mock_remote.name = "origin"

        # Create mock remote refs
        mock_remote_ref = MagicMock()
        mock_remote_ref.name = "origin/feature"
        mock_remote_ref.commit.hexsha = "ghi789"

        mock_head_ref = MagicMock()
        mock_head_ref.name = "origin/HEAD"

        mock_remote.refs = [mock_remote_ref, mock_head_ref]
        mock_repo.remotes = [mock_remote]

        mock_repo_class.return_value = mock_repo

        result = await git_adapter.get_all_branches(local_path)

        assert len(result) == 3  # main, develop, feature

        # Check local branches
        main_branch = next(b for b in result if b["name"] == "main")
        assert main_branch["type"] == "local"
        assert main_branch["head_commit_sha"] == "abc123"
        assert main_branch["is_active"] is True

        dev_branch = next(b for b in result if b["name"] == "develop")
        assert dev_branch["type"] == "local"
        assert dev_branch["head_commit_sha"] == "def456"
        assert dev_branch["is_active"] is False

        # Check remote branch
        feature_branch = next(b for b in result if b["name"] == "feature")
        assert feature_branch["type"] == "remote"
        assert feature_branch["head_commit_sha"] == "ghi789"
        assert feature_branch["is_active"] is False
        assert feature_branch["remote"] == "origin"


@pytest.mark.asyncio
async def test_get_branch_commits_local_branch(
    git_adapter: GitPythonAdapter, mock_commit: MagicMock
) -> None:
    """Test getting commits for a local branch."""
    local_path = Path("/tmp/test-repo")
    branch_name = "main"

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_branch = MagicMock()
        mock_repo = MagicMock()

        # Mock branches to return the branch when accessed
        def mock_branches_getitem(key: str) -> MagicMock:
            if key == branch_name:
                return mock_branch
            raise IndexError(f"No branch named {key}")

        mock_repo.branches.__getitem__.side_effect = mock_branches_getitem
        mock_repo.iter_commits.return_value = [mock_commit]
        mock_repo_class.return_value = mock_repo

        result = await git_adapter.get_branch_commits(local_path, branch_name)

        assert len(result) == 1
        commit = result[0]
        assert commit["sha"] == "abc123def456"
        assert commit["message"] == "Test commit message"
        assert commit["author_name"] == "Test Author"
        assert commit["author_email"] == "test@example.com"
        assert commit["parent_sha"] == ""
        mock_repo.iter_commits.assert_called_once_with(mock_branch)


@pytest.mark.asyncio
async def test_get_branch_commits_remote_branch(
    git_adapter: GitPythonAdapter, mock_commit: MagicMock
) -> None:
    """Test getting commits for a remote branch."""
    local_path = Path("/tmp/test-repo")
    branch_name = "feature"

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo = MagicMock()

        # Mock branches to raise IndexError when accessing non-existent branch
        def mock_getitem(key: str) -> Any:
            raise IndexError(f"No branch named {key}")

        mock_repo.branches.__getitem__.side_effect = mock_getitem

        mock_remote = MagicMock()
        mock_remote_ref = MagicMock()

        # Mock remote refs to return the ref when accessed
        def mock_refs_getitem(key: str) -> MagicMock:
            if key == branch_name:
                return mock_remote_ref
            raise IndexError(f"No ref named {key}")

        mock_remote.refs.__getitem__.side_effect = mock_refs_getitem

        mock_repo.remotes = [mock_remote]
        mock_repo.iter_commits.return_value = [mock_commit]
        mock_repo_class.return_value = mock_repo

        result = await git_adapter.get_branch_commits(local_path, branch_name)

        assert len(result) == 1
        mock_repo.iter_commits.assert_called_once_with(mock_remote_ref)


@pytest.mark.asyncio
async def test_get_branch_commits_branch_not_found(
    git_adapter: GitPythonAdapter,
) -> None:
    """Test error handling when branch is not found."""
    local_path = Path("/tmp/test-repo")
    branch_name = "nonexistent"

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo = MagicMock()

        # Mock branches to raise IndexError when accessing non-existent branch
        def mock_branches_getitem(key: str) -> Any:
            raise IndexError(f"No branch named {key}")

        mock_repo.branches.__getitem__.side_effect = mock_branches_getitem

        # Mock remotes with no refs for the branch
        mock_remote = MagicMock()

        def mock_refs_getitem(key: str) -> Any:
            raise IndexError(f"No ref named {key}")

        mock_remote.refs.__getitem__.side_effect = mock_refs_getitem

        mock_repo.remotes = [mock_remote]
        mock_repo_class.return_value = mock_repo

        with pytest.raises(ValueError, match="Branch nonexistent not found"):
            await git_adapter.get_branch_commits(local_path, branch_name)


@pytest.mark.asyncio
async def test_get_commit_files(git_adapter: GitPythonAdapter) -> None:
    """Test getting files from a specific commit."""
    local_path = Path("/tmp/test-repo")
    commit_sha = "abc123"

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        # Create a simple mock that skips the Blob type check
        mock_commit = MagicMock()

        # Create a simple tree with no items to traverse
        mock_tree = MagicMock()
        mock_tree.traverse.return_value = []
        mock_commit.tree = mock_tree
        mock_commit.committed_datetime = datetime.now(UTC)

        mock_repo = MagicMock()
        mock_repo.commit.return_value = mock_commit
        mock_repo_class.return_value = mock_repo

        result = await git_adapter.get_commit_files(local_path, commit_sha, mock_repo)

        # Should return empty list since no blobs
        assert result == []


@pytest.mark.asyncio
async def test_repository_exists_true(git_adapter: GitPythonAdapter) -> None:
    """Test repository exists check when repo is valid."""
    local_path = Path("/tmp/test-repo")

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo_class.return_value = MagicMock()

        result = await git_adapter.repository_exists(local_path)

        assert result is True
        mock_repo_class.assert_called_once_with(local_path)


@pytest.mark.asyncio
async def test_repository_exists_false(git_adapter: GitPythonAdapter) -> None:
    """Test repository exists check when repo is invalid."""
    local_path = Path("/tmp/test-repo")

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo_class.side_effect = InvalidGitRepositoryError("Not a git repository")

        result = await git_adapter.repository_exists(local_path)

        assert result is False


@pytest.mark.asyncio
async def test_get_commit_details(
    git_adapter: GitPythonAdapter, mock_commit: MagicMock
) -> None:
    """Test getting detailed commit information."""
    local_path = Path("/tmp/test-repo")
    commit_sha = "abc123"

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo = MagicMock()
        mock_repo.commit.return_value = mock_commit
        mock_repo_class.return_value = mock_repo

        result = await git_adapter.get_commit_details(local_path, commit_sha)

        assert result["sha"] == "abc123def456"
        assert result["message"] == "Test commit message"
        assert result["author_name"] == "Test Author"
        assert result["author_email"] == "test@example.com"
        assert result["parent_sha"] == ""
        assert result["stats"] == {
            "insertions": 10,
            "deletions": 5,
            "lines": 15,
            "files": 2,
        }
        assert isinstance(result["date"], datetime)


@pytest.mark.asyncio
async def test_ensure_repository_exists(git_adapter: GitPythonAdapter) -> None:
    """Test ensure_repository when repository already exists."""
    remote_uri = "https://github.com/test/repo.git"
    local_path = Path("/tmp/test-repo")

    with (
        patch.object(git_adapter, "repository_exists", return_value=True),
        patch.object(git_adapter, "pull_repository") as mock_pull,
        patch.object(git_adapter, "clone_repository") as mock_clone,
    ):
        await git_adapter.ensure_repository(remote_uri, local_path)

        mock_pull.assert_called_once_with(local_path)
        mock_clone.assert_not_called()


@pytest.mark.asyncio
async def test_ensure_repository_not_exists(git_adapter: GitPythonAdapter) -> None:
    """Test ensure_repository when repository doesn't exist."""
    remote_uri = "https://github.com/test/repo.git"
    local_path = Path("/tmp/test-repo")

    with (
        patch.object(git_adapter, "repository_exists", return_value=False),
        patch.object(git_adapter, "pull_repository") as mock_pull,
        patch.object(git_adapter, "clone_repository") as mock_clone,
    ):
        await git_adapter.ensure_repository(remote_uri, local_path)

        mock_clone.assert_called_once_with(remote_uri, local_path)
        mock_pull.assert_not_called()


@pytest.mark.asyncio
async def test_get_file_content(git_adapter: GitPythonAdapter) -> None:
    """Test getting file content at specific commit."""
    local_path = Path("/tmp/test-repo")
    commit_sha = "abc123"
    file_path = "src/main.py"
    expected_content = b"print('Hello, World!')"

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_blob = MagicMock()
        mock_blob.data_stream.read.return_value = expected_content

        mock_commit = MagicMock()
        mock_commit.tree = {file_path: mock_blob}

        mock_repo = MagicMock()
        mock_repo.commit.return_value = mock_commit
        mock_repo_class.return_value = mock_repo

        result = await git_adapter.get_file_content(local_path, commit_sha, file_path)

        assert result == expected_content


@pytest.mark.asyncio
async def test_get_latest_commit_sha_head(git_adapter: GitPythonAdapter) -> None:
    """Test getting latest commit SHA for HEAD."""
    local_path = Path("/tmp/test-repo")
    expected_sha = "abc123def456"

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo = MagicMock()
        mock_repo.head.commit.hexsha = expected_sha
        mock_repo_class.return_value = mock_repo

        result = await git_adapter.get_latest_commit_sha(local_path)

        assert result == expected_sha


@pytest.mark.asyncio
async def test_get_latest_commit_sha_specific_branch(
    git_adapter: GitPythonAdapter,
) -> None:
    """Test getting latest commit SHA for specific branch."""
    local_path = Path("/tmp/test-repo")
    branch_name = "develop"
    expected_sha = "def456ghi789"

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_branch = MagicMock()
        mock_branch.commit.hexsha = expected_sha

        mock_repo = MagicMock()

        # Mock branches to return the branch when accessed
        def mock_branches_getitem(key: str) -> MagicMock:
            if key == branch_name:
                return mock_branch
            raise IndexError(f"No branch named {key}")

        mock_repo.branches.__getitem__.side_effect = mock_branches_getitem

        mock_repo_class.return_value = mock_repo

        result = await git_adapter.get_latest_commit_sha(local_path, branch_name)

        assert result == expected_sha


@pytest.mark.asyncio
async def test_get_all_tags(git_adapter: GitPythonAdapter) -> None:
    """Test getting all repository tags."""
    local_path = Path("/tmp/test-repo")

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        # Create mock tags
        mock_tag1 = MagicMock()
        mock_tag1.name = "v1.0.0"
        mock_tag1.commit.hexsha = "tag1sha"

        mock_tag2 = MagicMock()
        mock_tag2.name = "v2.0.0"
        mock_tag2.commit.hexsha = "tag2sha"

        mock_repo = MagicMock()
        mock_repo.tags = [mock_tag1, mock_tag2]
        mock_repo_class.return_value = mock_repo

        result = await git_adapter.get_all_tags(local_path)

        assert len(result) == 2
        assert result[0]["name"] == "v1.0.0"
        assert result[0]["target_commit_sha"] == "tag1sha"
        assert result[1]["name"] == "v2.0.0"
        assert result[1]["target_commit_sha"] == "tag2sha"


@pytest.mark.asyncio
async def test_executor_cleanup() -> None:
    """Test that executor is properly cleaned up."""
    adapter = GitPythonAdapter(max_workers=2)

    # Mock the executor to track shutdown calls
    with patch.object(adapter.executor, "shutdown") as mock_shutdown:
        del adapter
        mock_shutdown.assert_called_once_with(wait=True)


@pytest.mark.asyncio
async def test_concurrent_operations(git_adapter: GitPythonAdapter) -> None:
    """Test that multiple operations can run concurrently."""
    local_path1 = Path("/tmp/test-repo1")
    local_path2 = Path("/tmp/test-repo2")

    with patch(f"{MODULE_PATH}.Repo") as mock_repo_class:
        mock_repo_class.return_value = MagicMock()

        # Run multiple operations concurrently
        tasks = [
            git_adapter.repository_exists(local_path1),
            git_adapter.repository_exists(local_path2),
        ]

        results = await asyncio.gather(*tasks)

        assert all(results)
        assert mock_repo_class.call_count == 2


@pytest.mark.asyncio
async def test_get_default_branch_main(
    git_adapter: GitPythonAdapter,
) -> None:
    """Test get_default_branch returns main branch."""
    import tempfile

    from git import Repo as GitRepo

    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        # Initialize a git repo with main as default branch
        repo = GitRepo.init(repo_path, initial_branch="main")

        # Create an initial commit
        test_file = repo_path / "README.md"
        test_file.write_text("# Test Repo")
        repo.index.add(["README.md"])
        repo.index.commit("Initial commit")

        # Add a fake remote origin
        repo.create_remote("origin", "https://github.com/test/repo.git")

        # Set origin/HEAD to point to origin/main
        # This simulates what happens when you clone a repo
        repo.git.symbolic_ref("refs/remotes/origin/HEAD", "refs/remotes/origin/main")

        result = await git_adapter.get_default_branch(repo_path)

        assert result == "main"


@pytest.mark.asyncio
async def test_get_default_branch_master(
    git_adapter: GitPythonAdapter,
) -> None:
    """Test get_default_branch returns master branch."""
    import tempfile

    from git import Repo as GitRepo

    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        # Initialize a git repo with master as default branch
        repo = GitRepo.init(repo_path, initial_branch="master")

        # Create an initial commit
        test_file = repo_path / "README.md"
        test_file.write_text("# Test Repo")
        repo.index.add(["README.md"])
        repo.index.commit("Initial commit")

        # Add a fake remote origin
        repo.create_remote("origin", "https://github.com/test/repo.git")

        # Set origin/HEAD to point to origin/master
        # This simulates what happens when you clone a repo
        repo.git.symbolic_ref("refs/remotes/origin/HEAD", "refs/remotes/origin/master")

        result = await git_adapter.get_default_branch(repo_path)

        assert result == "master"
