"""Tests for Git repository service domain classes."""

from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest
from pydantic import AnyUrl

from kodit.domain.entities.git import GitBranch, GitCommit, GitRepo
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.protocols import GitAdapter
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
    RepositoryInfo,
    RepositoryScanResult,
)


@pytest.fixture
def mock_git_adapter() -> AsyncMock:
    """Create a mock GitAdapter."""
    return AsyncMock(spec=GitAdapter)


@pytest.fixture
def sample_branch_data() -> list[dict]:
    """Sample branch data from GitAdapter."""
    return [
        {"name": "main", "type": "local", "head_commit_sha": "abc123"},
        {"name": "develop", "type": "local", "head_commit_sha": "def456"},
    ]


@pytest.fixture
def sample_commit_data() -> list[dict]:
    """Sample commit data from GitAdapter."""
    return [
        {
            "sha": "abc123",
            "date": datetime.now(UTC),
            "message": "Initial commit",
            "parent_sha": "",
            "author_name": "Test Author",
            "author_email": "test@example.com",
        }
    ]


@pytest.fixture
def sample_develop_commit_data() -> list[dict]:
    """Sample commit data for develop branch from GitAdapter."""
    return [
        {
            "sha": "def456",
            "date": datetime.now(UTC),
            "message": "Second commit",
            "parent_sha": "abc123",
            "author_name": "Test Author",
            "author_email": "test@example.com",
        }
    ]


@pytest.fixture
def sample_file_data() -> list[dict]:
    """Sample file data from GitAdapter."""
    return [
        {
            "blob_sha": "file123",
            "path": "src/main.py",
            "mime_type": "text/x-python",
            "size": 1024,
        }
    ]


@pytest.fixture
def sample_tag_data() -> list[dict]:
    """Sample tag data from GitAdapter."""
    return [
        {"name": "v1.0.0", "target_commit_sha": "abc123"},
    ]


@pytest.mark.asyncio
async def test_git_repository_scanner_scan_repository(
    mock_git_adapter: AsyncMock,
    sample_branch_data: list[dict],
    sample_commit_data: list[dict],
    sample_file_data: list[dict],
    sample_tag_data: list[dict],
) -> None:
    """Test GitRepositoryScanner.scan_repository."""
    # Setup mock responses
    mock_git_adapter.get_all_branches.return_value = sample_branch_data
    mock_git_adapter.get_branch_commits.return_value = sample_commit_data
    mock_git_adapter.get_commit_files.return_value = sample_file_data
    mock_git_adapter.get_all_tags.return_value = sample_tag_data

    # Setup new bulk operations - using sample_develop_commit_data for def456
    sample_develop_commit = {
        "sha": "def456",
        "date": datetime.now(UTC),
        "message": "Second commit",
        "parent_sha": "abc123",
        "author_name": "Test Author",
        "author_email": "test@example.com",
    }
    mock_git_adapter.get_all_commits_bulk.return_value = {
        "abc123": sample_commit_data[0],
        "def456": sample_develop_commit,
    }
    mock_git_adapter.get_branch_commit_shas.return_value = ["abc123", "def456"]

    scanner = GitRepositoryScanner(mock_git_adapter)
    cloned_path = Path("/tmp/test-repo")

    result = await scanner.scan_repository(cloned_path)

    # Verify result structure
    assert isinstance(result, RepositoryScanResult)
    assert len(result.branches) == 2
    assert len(result.all_commits) == 2
    assert len(result.all_tags) == 1
    # Files loaded on-demand for performance optimization
    assert result.total_files_across_commits == 0

    # Verify branches
    branch_names = {branch.name for branch in result.branches}
    assert "main" in branch_names
    assert "develop" in branch_names

    # Verify tags
    tag_names = {tag.name for tag in result.all_tags}
    assert "v1.0.0" in tag_names


@pytest.mark.asyncio
async def test_git_repository_scanner_empty_branch(
    mock_git_adapter: AsyncMock,
) -> None:
    """Test GitRepositoryScanner with empty branch."""
    mock_git_adapter.get_all_branches.return_value = [
        {"name": "empty", "type": "local", "head_commit_sha": "abc123"}
    ]
    mock_git_adapter.get_branch_commits.return_value = []  # Empty branch
    mock_git_adapter.get_all_tags.return_value = []

    # Setup new bulk operations for empty repository
    mock_git_adapter.get_all_commits_bulk.return_value = {}
    mock_git_adapter.get_branch_commit_shas.return_value = []

    scanner = GitRepositoryScanner(mock_git_adapter)
    result = await scanner.scan_repository(Path("/tmp/test-repo"))

    assert len(result.branches) == 0  # Empty branch should be filtered out
    assert len(result.all_commits) == 0


@pytest.mark.asyncio
async def test_git_repository_scanner_malformed_tag(
    mock_git_adapter: AsyncMock,
    sample_branch_data: list[dict],
    sample_commit_data: list[dict],
    sample_file_data: list[dict],
) -> None:
    """Test GitRepositoryScanner handles malformed tags gracefully."""
    mock_git_adapter.get_all_branches.return_value = sample_branch_data
    mock_git_adapter.get_branch_commits.return_value = sample_commit_data
    mock_git_adapter.get_commit_files.return_value = sample_file_data

    # Malformed tag data missing required fields
    mock_git_adapter.get_all_tags.return_value = [
        {"name": "v1.0.0", "target_commit_sha": "abc123"},
        {"malformed": "tag"},  # Missing required fields
    ]

    # Setup new bulk operations - using sample_develop_commit_data for def456
    sample_develop_commit = {
        "sha": "def456",
        "date": datetime.now(UTC),
        "message": "Second commit",
        "parent_sha": "abc123",
        "author_name": "Test Author",
        "author_email": "test@example.com",
    }
    mock_git_adapter.get_all_commits_bulk.return_value = {
        "abc123": sample_commit_data[0],
        "def456": sample_develop_commit,
    }
    mock_git_adapter.get_branch_commit_shas.return_value = ["abc123", "def456"]

    scanner = GitRepositoryScanner(mock_git_adapter)
    result = await scanner.scan_repository(Path("/tmp/test-repo"))

    assert len(result.all_tags) == 1  # Only valid tag should be included
    assert result.all_tags[0].name == "v1.0.0"


def test_git_repo_factory_create_from_scan() -> None:
    """Test GitRepoFactory.create_from_scan."""
    # Create sample data
    repo_info = RepositoryInfo(
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo.git"),
        cloned_path=Path("/tmp/test-repo"),
    )

    main_branch = GitBranch(
        name="main",
        head_commit=GitCommit(
            commit_sha="abc123",
            date=datetime.now(UTC),
            message="Test commit",
            parent_commit_sha="",
            files=[],
            author="Test Author <test@example.com>",
        ),
    )

    scan_result = RepositoryScanResult(
        branches=[main_branch],
        all_commits=[main_branch.head_commit],
        all_tags=[],
        scan_timestamp=datetime.now(UTC),
        total_files_across_commits=0,
    )

    # Create GitRepo
    git_repo = GitRepoFactory.create_from_remote_uri(repo_info.remote_uri)
    git_repo.update_with_scan_result(scan_result)

    assert isinstance(git_repo, GitRepo)
    assert git_repo.remote_uri == repo_info.remote_uri
    assert git_repo.sanitized_remote_uri == repo_info.sanitized_remote_uri
    assert git_repo.tracking_branch == main_branch
    assert git_repo.num_branches == 1


def test_git_repo_factory_prefers_main_branch() -> None:
    """Test GitRepoFactory prefers 'main' over other branches."""
    repo_info = RepositoryInfo(
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo.git"),
        cloned_path=Path("/tmp/test-repo"),
    )

    commit = GitCommit(
        commit_sha="abc123",
        date=datetime.now(UTC),
        message="Test commit",
        parent_commit_sha="",
        files=[],
        author="Test Author <test@example.com>",
    )

    # Create branches with main, master, and develop
    branches = [
        GitBranch(name="develop", head_commit=commit),
        GitBranch(name="master", head_commit=commit),
        GitBranch(name="main", head_commit=commit),
    ]

    git_repo = GitRepoFactory.create_from_remote_uri(repo_info.remote_uri)

    scan_result = RepositoryScanResult(
        branches=branches,
        all_commits=[commit],
        all_tags=[],
        scan_timestamp=datetime.now(UTC),
        total_files_across_commits=0,
    )
    git_repo.update_with_scan_result(scan_result)

    assert git_repo.tracking_branch is not None
    assert git_repo.tracking_branch.name == "main"


def test_git_repo_factory_no_branches_raises_error() -> None:
    """Test GitRepoFactory raises error when no branches available."""
    repo_info = RepositoryInfo(
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo.git"),
        cloned_path=Path("/tmp/test-repo"),
    )

    scan_result = RepositoryScanResult(
        branches=[],
        all_commits=[],
        all_tags=[],
        scan_timestamp=datetime.now(UTC),
        total_files_across_commits=0,
    )

    git_repo = GitRepoFactory.create_from_remote_uri(repo_info.remote_uri)

    with pytest.raises(ValueError, match="No tracking branch found"):
        git_repo.update_with_scan_result(scan_result)


@pytest.mark.asyncio
async def test_repository_cloner_clone_repository(
    mock_git_adapter: AsyncMock,
) -> None:
    """Test RepositoryCloner.clone_repository."""
    mock_git_adapter.clone_repository.return_value = None

    clone_dir = Path("/tmp/clones")
    cloner = RepositoryCloner(mock_git_adapter, clone_dir)
    remote_uri = AnyUrl("https://github.com/test/repo.git")

    result = await cloner.clone_repository(remote_uri)

    assert isinstance(result, Path)
    assert result == clone_dir / GitRepo.create_id(remote_uri)

    mock_git_adapter.clone_repository.assert_called_once()


@pytest.mark.asyncio
async def test_repository_cloner_clone_failure_cleanup(
    mock_git_adapter: AsyncMock,
) -> None:
    """Test RepositoryCloner cleans up on clone failure."""
    from unittest.mock import patch

    mock_git_adapter.clone_repository.side_effect = Exception("Clone failed")

    cloner = RepositoryCloner(mock_git_adapter, Path("/tmp/clones"))
    remote_uri = AnyUrl("https://github.com/test/repo.git")

    # Mock shutil.rmtree to avoid trying to remove non-existent directory
    with patch(
        "kodit.domain.services.git_repository_service.shutil.rmtree"
    ) as mock_rmtree:
        with pytest.raises(Exception, match="Clone failed"):
            await cloner.clone_repository(remote_uri)

        # Verify cleanup was attempted
        mock_rmtree.assert_called_once()


@pytest.mark.asyncio
async def test_repository_cloner_pull_repository_exists(
    mock_git_adapter: AsyncMock,
    tmp_path: Path,
) -> None:
    """Test RepositoryCloner.pull_repository when repository exists."""
    mock_git_adapter.pull_repository.return_value = None

    cloner = RepositoryCloner(mock_git_adapter, Path("/tmp/clones"))

    # Create a real temporary directory to avoid mocking issues
    existing_repo_path = tmp_path / "existing-repo"
    existing_repo_path.mkdir()

    # Mock GitRepo with existing path
    mock_repo = MagicMock(spec=GitRepo)
    mock_repo.cloned_path = existing_repo_path
    mock_repo.remote_uri = AnyUrl("https://github.com/test/repo.git")

    await cloner.pull_repository(mock_repo)

    mock_git_adapter.pull_repository.assert_called_once_with(existing_repo_path)


@pytest.mark.asyncio
async def test_repository_cloner_pull_repository_missing(
    mock_git_adapter: AsyncMock,
    tmp_path: Path,
) -> None:
    """Test RepositoryCloner.pull_repository when repository is missing."""
    mock_git_adapter.clone_repository.return_value = None

    cloner = RepositoryCloner(mock_git_adapter, Path("/tmp/clones"))

    # Use a path that doesn't exist
    missing_repo_path = tmp_path / "missing-repo"  # Don't create this directory

    # Mock GitRepo with non-existing path
    mock_repo = MagicMock(spec=GitRepo)
    mock_repo.cloned_path = missing_repo_path
    mock_repo.remote_uri = AnyUrl("https://github.com/test/repo.git")

    await cloner.pull_repository(mock_repo)

    mock_git_adapter.clone_repository.assert_called_once()
