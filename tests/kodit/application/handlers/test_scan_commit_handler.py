"""Tests for ScanCommitHandler."""

import tempfile
from collections.abc import Callable
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import git
import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.handlers.commit.scan_commit import ScanCommitHandler
from kodit.application.services.reporting import ProgressTracker
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_file_repository import (
    create_git_file_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


@pytest.fixture
def mock_progress() -> MagicMock:
    """Create a mock progress tracker."""
    tracker = MagicMock(spec=ProgressTracker)
    context_manager = AsyncMock()
    context_manager.__aenter__ = AsyncMock(return_value=context_manager)
    context_manager.__aexit__ = AsyncMock(return_value=None)
    context_manager.skip = AsyncMock()
    tracker.create_child = MagicMock(return_value=context_manager)
    return tracker


@pytest.fixture
async def scan_handler(
    session_factory: Callable[[], AsyncSession],
    mock_progress: MagicMock,
) -> ScanCommitHandler:
    """Create a ScanCommitHandler instance."""
    git_adapter = GitPythonAdapter()
    scanner = GitRepositoryScanner(git_adapter)

    return ScanCommitHandler(
        repo_repository=create_git_repo_repository(session_factory),
        git_commit_repository=create_git_commit_repository(session_factory),
        git_file_repository=create_git_file_repository(session_factory),
        scanner=scanner,
        operation=mock_progress,
    )


@pytest.mark.asyncio
async def test_scan_commit_saves_commit_and_files(
    scan_handler: ScanCommitHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that scanning a commit saves commit and files to database."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        # Create real git repo with files
        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        # Create multiple files
        (repo_path / "file1.py").write_text("print('hello')")
        (repo_path / "file2.js").write_text("console.log('hello');")
        git_repo.index.add(["file1.py", "file2.js"])
        commit = git_repo.index.commit("Add files")

        # Setup repository in database
        repo_repository = create_git_repo_repository(session_factory)
        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        # Scan the commit
        await scan_handler.execute({
            "repository_id": repo.id,
            "commit_sha": commit.hexsha,
        })

        # Verify commit was saved
        commit_repo = create_git_commit_repository(session_factory)
        saved_commit = await commit_repo.get(commit.hexsha)
        assert saved_commit is not None
        assert saved_commit.commit_sha == commit.hexsha
        assert saved_commit.repo_id == repo.id

        # Verify files were saved
        file_repo = create_git_file_repository(session_factory)
        files = await file_repo.find(
            QueryBuilder().filter("commit_sha", FilterOperator.EQ, commit.hexsha)
        )
        assert len(files) == 2
        file_names = {Path(f.path).name for f in files}
        assert "file1.py" in file_names
        assert "file2.js" in file_names


@pytest.mark.asyncio
async def test_scan_commit_is_idempotent(
    scan_handler: ScanCommitHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that scanning the same commit twice doesn't create duplicates."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        (repo_path / "file.txt").write_text("content")
        git_repo.index.add(["file.txt"])
        commit = git_repo.index.commit("Initial commit")

        repo_repository = create_git_repo_repository(session_factory)
        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        payload = {"repository_id": repo.id, "commit_sha": commit.hexsha}

        # Scan first time
        await scan_handler.execute(payload)

        # Scan second time - should skip
        await scan_handler.execute(payload)

        # Verify only one commit exists
        commit_repo = create_git_commit_repository(session_factory)
        commits = await commit_repo.find(
            QueryBuilder().filter("commit_sha", FilterOperator.EQ, commit.hexsha)
        )
        assert len(commits) == 1


@pytest.mark.asyncio
async def test_scan_commit_updates_repository_metadata(
    scan_handler: ScanCommitHandler,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that scanning updates repository last_scanned_at and num_commits."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        (repo_path / "file.txt").write_text("content")
        git_repo.index.add(["file.txt"])
        commit = git_repo.index.commit("Initial commit")

        repo_repository = create_git_repo_repository(session_factory)
        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        initial_scanned_at = repo.last_scanned_at

        # Scan commit
        await scan_handler.execute({
            "repository_id": repo.id,
            "commit_sha": commit.hexsha,
        })

        # Verify metadata was updated
        updated_repo = await repo_repository.get(repo.id)
        assert updated_repo.last_scanned_at != initial_scanned_at
        assert updated_repo.last_scanned_at is not None
        assert updated_repo.num_commits == 1
