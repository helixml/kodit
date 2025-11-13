"""Tests for RepositorySyncService."""

import tempfile
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import git
import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.repository_sync_service import RepositorySyncService
from kodit.domain.entities.git import GitCommit
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.git_repository_service import GitRepositoryScanner
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


@pytest.fixture
async def sync_service(
    session_factory: Callable[[], AsyncSession],
) -> RepositorySyncService:
    """Create a RepositorySyncService instance."""
    git_adapter = GitPythonAdapter()
    scanner = GitRepositoryScanner(git_adapter)

    return RepositorySyncService(
        scanner=scanner,
        git_commit_repository=create_git_commit_repository(session_factory),
        git_branch_repository=create_git_branch_repository(session_factory),
        git_tag_repository=create_git_tag_repository(session_factory),
    )


@pytest.mark.asyncio
async def test_sync_creates_branches_and_tags(
    sync_service: RepositorySyncService,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test syncing branches and tags from a real git repo."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        # Create real git repo
        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        # Create commits
        test_file = repo_path / "test.txt"
        test_file.write_text("initial")
        git_repo.index.add(["test.txt"])
        commit1 = git_repo.index.commit("Initial commit")

        # Create branch
        git_repo.create_head("feature", commit1.hexsha)

        test_file.write_text("updated")
        git_repo.index.add(["test.txt"])
        commit2 = git_repo.index.commit("Second commit")

        # Create tag
        git_repo.create_tag("v1.0.0", ref=commit2.hexsha)

        # Setup database
        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)
        branch_repository = create_git_branch_repository(session_factory)
        tag_repository = create_git_tag_repository(session_factory)

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        # Save commits to database (FK requirement)
        for commit_obj in [commit1, commit2]:
            commit = GitCommit(
                commit_sha=commit_obj.hexsha,
                repo_id=repo.id,
                message=str(commit_obj.message),
                author=str(commit_obj.author),
                date=datetime.fromtimestamp(commit_obj.committed_date, UTC),
            )
            await commit_repository.save(commit)

        # Sync branches and tags
        await sync_service.sync_branches_and_tags(repo)

        # Verify branches
        branches = await branch_repository.get_by_repo_id(repo.id)
        assert len(branches) == 2  # main/master + feature
        branch_names = {b.name for b in branches}
        assert "feature" in branch_names

        # Verify tags
        tags = await tag_repository.get_by_repo_id(repo.id)
        assert len(tags) == 1
        assert tags[0].name == "v1.0.0"
        assert tags[0].target_commit_sha == commit2.hexsha


@pytest.mark.asyncio
async def test_sync_handles_missing_commits(
    sync_service: RepositorySyncService,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that sync creates branches/tags even when commits aren't indexed yet."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        test_file = repo_path / "test.txt"
        test_file.write_text("initial")
        git_repo.index.add(["test.txt"])
        commit1 = git_repo.index.commit("Initial commit")

        git_repo.create_head("branch-with-unscanned-commit", commit1.hexsha)
        git_repo.create_tag("tag-with-unscanned-commit", ref=commit1.hexsha)

        # Setup database but DON'T save commits
        repo_repository = create_git_repo_repository(session_factory)
        branch_repository = create_git_branch_repository(session_factory)
        tag_repository = create_git_tag_repository(session_factory)

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        # Sync should not fail even though commits don't exist
        await sync_service.sync_branches_and_tags(repo)

        # Verify branches/tags were created even though commits don't exist in DB yet
        branches = await branch_repository.get_by_repo_id(repo.id)
        tags = await tag_repository.get_by_repo_id(repo.id)

        # Branches/tags are created even without indexed commits (FK constraint removed)
        assert len(branches) == 2  # main + branch-with-unscanned-commit
        assert len(tags) == 1  # tag-with-unscanned-commit
        assert branches[0].head_commit_sha == commit1.hexsha
        assert tags[0].target_commit_sha == commit1.hexsha


@pytest.mark.asyncio
async def test_sync_deletes_stale_branches_and_tags(
    sync_service: RepositorySyncService,
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that deleted branches/tags in git are removed from database."""
    with tempfile.TemporaryDirectory() as tmpdir:
        repo_path = Path(tmpdir) / "test-repo"
        repo_path.mkdir()

        git_repo = git.Repo.init(repo_path)
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        test_file = repo_path / "test.txt"
        test_file.write_text("initial")
        git_repo.index.add(["test.txt"])
        commit1 = git_repo.index.commit("Initial commit")

        # Create and then delete branch/tag
        temp_branch = git_repo.create_head("temp-branch", commit1.hexsha)
        temp_tag = git_repo.create_tag("temp-tag", ref=commit1.hexsha)

        # Setup database
        repo_repository = create_git_repo_repository(session_factory)
        commit_repository = create_git_commit_repository(session_factory)
        branch_repository = create_git_branch_repository(session_factory)
        tag_repository = create_git_tag_repository(session_factory)

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        # Save commit
        commit = GitCommit(
            commit_sha=commit1.hexsha,
            repo_id=repo.id,
            message=str(commit1.message),
            author=str(commit1.author),
            date=datetime.fromtimestamp(commit1.committed_date, UTC),
        )
        await commit_repository.save(commit)

        # First sync - creates branch/tag
        await sync_service.sync_branches_and_tags(repo)

        branches_before = await branch_repository.get_by_repo_id(repo.id)
        tags_before = await tag_repository.get_by_repo_id(repo.id)
        assert any(b.name == "temp-branch" for b in branches_before)
        assert any(t.name == "temp-tag" for t in tags_before)

        # Delete from git
        git_repo.delete_head(temp_branch, force=True)
        git_repo.delete_tag(temp_tag)

        # Second sync - should delete from database
        await sync_service.sync_branches_and_tags(repo)

        branches_after = await branch_repository.get_by_repo_id(repo.id)
        tags_after = await tag_repository.get_by_repo_id(repo.id)
        assert not any(b.name == "temp-branch" for b in branches_after)
        assert not any(t.name == "temp-tag" for t in tags_after)
