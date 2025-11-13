"""Tests for SyncRepositoryHandler."""

import tempfile
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import git
import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.handlers.repository.sync_repository import (
    SyncRepositoryHandler,
)
from kodit.application.services.queue_service import QueueService
from kodit.application.services.reporting import ProgressTracker
from kodit.application.services.repository_query_service import (
    RepositoryQueryService,
)
from kodit.application.services.repository_sync_service import RepositorySyncService
from kodit.domain.entities.git import GitCommit, TrackingConfig
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.sqlalchemy.git_branch_repository import (
    create_git_branch_repository,
)
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.git_tag_repository import (
    create_git_tag_repository,
)
from kodit.infrastructure.sqlalchemy.query import QueryBuilder


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
def mock_queue() -> MagicMock:
    """Create a mock queue service."""
    queue = MagicMock(spec=QueueService)
    queue.enqueue_tasks = AsyncMock()
    return queue


@pytest.fixture
async def sync_handler(
    session_factory: Callable[[], AsyncSession],
    mock_progress: MagicMock,
    mock_queue: MagicMock,
    tmp_path: Path,
) -> SyncRepositoryHandler:
    """Create a SyncRepositoryHandler instance."""
    git_adapter = GitPythonAdapter()
    scanner = GitRepositoryScanner(git_adapter)
    clone_dir = tmp_path / "clones"
    clone_dir.mkdir()
    cloner = RepositoryCloner(git_adapter, clone_dir)

    git_commit_repository = create_git_commit_repository(session_factory)
    git_branch_repository = create_git_branch_repository(session_factory)
    git_tag_repository = create_git_tag_repository(session_factory)
    repo_repository = create_git_repo_repository(session_factory)

    sync_service = RepositorySyncService(
        scanner=scanner,
        git_commit_repository=git_commit_repository,
        git_branch_repository=git_branch_repository,
        git_tag_repository=git_tag_repository,
    )

    trackable_resolution = TrackableResolutionService(
        commit_repo=git_commit_repository,
        branch_repo=git_branch_repository,
        tag_repo=git_tag_repository,
    )

    query_service = RepositoryQueryService(
        git_repo_repository=repo_repository,
        trackable_resolution=trackable_resolution,
        git_adapter=git_adapter,
    )

    return SyncRepositoryHandler(
        repo_repository=repo_repository,
        git_commit_repository=git_commit_repository,
        cloner=cloner,
        repository_sync_service=sync_service,
        repository_query_service=query_service,
        queue=mock_queue,
        operation=mock_progress,
    )


@pytest.mark.asyncio
async def test_sync_repository_with_new_commit_on_main(  # noqa: PLR0915
    sync_handler: SyncRepositoryHandler,
    session_factory: Callable[[], AsyncSession],
    mock_queue: MagicMock,
) -> None:
    """Test syncing repository when main has a new commit.

    This simulates the scenario where:
    1. A repository has been indexed with an initial commit
    2. A new commit is added to main
    3. Sync is called - branches/tags are correctly updated even though
       the new commit hasn't been indexed yet (FK constraint removed)
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create bare remote repository
        remote_path = Path(tmpdir) / "remote.git"
        git.Repo.init(remote_path, bare=True, initial_branch="main")

        # Create working repository cloned from remote
        repo_path = Path(tmpdir) / "repo"
        git_repo = git.Repo.clone_from(str(remote_path), str(repo_path))
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        # Create initial commit on main
        test_file = repo_path / "test.txt"
        test_file.write_text("initial")
        git_repo.index.add(["test.txt"])
        commit1 = git_repo.index.commit("Initial commit")

        # Push to remote - use HEAD:main to ensure we're creating the main branch
        git_repo.git.push("origin", "HEAD:main")

        # Setup database
        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)
        branch_repository = create_git_branch_repository(session_factory)
        tag_repository = create_git_tag_repository(session_factory)

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        # Index initial commit and sync branches
        initial_commit = GitCommit(
            commit_sha=commit1.hexsha,
            repo_id=repo.id,
            message=str(commit1.message),
            author=str(commit1.author),
            date=datetime.fromtimestamp(commit1.committed_date, UTC),
        )
        await commit_repository.save(initial_commit)

        # Sync branches for initial state
        sync_service = RepositorySyncService(
            scanner=GitRepositoryScanner(GitPythonAdapter()),
            git_commit_repository=commit_repository,
            git_branch_repository=branch_repository,
            git_tag_repository=tag_repository,
        )
        await sync_service.sync_branches_and_tags(repo)

        # Verify initial state - main branch exists
        initial_branches = await branch_repository.get_by_repo_id(repo.id)
        assert len(initial_branches) == 1
        assert initial_branches[0].name == "main"
        assert initial_branches[0].head_commit_sha == commit1.hexsha

        # Create new commit on main and push to remote
        test_file.write_text("updated")
        git_repo.index.add(["test.txt"])
        commit2 = git_repo.index.commit("Update on main")
        git_repo.create_tag("v1.0.0", ref=commit2.hexsha)
        git_repo.git.push("origin", "HEAD:main")
        git_repo.git.push("origin", "v1.0.0")

        # Execute sync - this demonstrates the bug
        await sync_handler.execute({"repository_id": repo.id})

        # Verify new commit is queued for indexing
        assert mock_queue.enqueue_tasks.called
        payload = mock_queue.enqueue_tasks.call_args.kwargs["payload"]
        assert payload["commit_sha"] == commit2.hexsha

        # FIXED: main branch is updated to point to commit2 even though it's not indexed
        branches_after = await branch_repository.get_by_repo_id(repo.id)
        assert len(branches_after) == 1
        assert branches_after[0].name == "main"
        assert branches_after[0].head_commit_sha == commit2.hexsha

        # FIXED: tag was created pointing to commit2 even though it's not indexed
        tags_after = await tag_repository.get_by_repo_id(repo.id)
        assert len(tags_after) == 1
        assert tags_after[0].name == "v1.0.0"
        assert tags_after[0].target_commit_sha == commit2.hexsha

        # New commit not indexed yet (it's only queued)
        commits_in_db = await commit_repository.find(QueryBuilder())
        commit_shas = {c.commit_sha for c in commits_in_db}
        assert commit2.hexsha not in commit_shas
