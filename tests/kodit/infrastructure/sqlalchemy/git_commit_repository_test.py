"""Tests for SqlAlchemyGitCommitRepository."""

from collections.abc import Callable
from datetime import UTC, datetime

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitFile, GitRepo
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository


@pytest.fixture
async def repo_with_commits(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
    sample_git_file: GitFile,
) -> tuple[GitRepo, list[GitCommit]]:
    """Create a repository with commits for testing."""
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)

    # Save repository
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None

    # Create commits
    commits = [
        GitCommit(
            created_at=datetime.now(UTC),
            commit_sha="commit1",
            date=datetime.now(UTC),
            message="First commit",
            parent_commit_sha=None,
            files=[sample_git_file],
            author="test@example.com",
        ),
        GitCommit(
            created_at=datetime.now(UTC),
            commit_sha="commit2",
            date=datetime.now(UTC),
            message="Second commit",
            parent_commit_sha="commit1",
            files=[],
            author="test@example.com",
        ),
    ]

    await commit_repository.save_bulk(commits, saved_repo.id)
    return saved_repo, commits


class TestCommitDeletion:
    """Test commit deletion functionality."""

    async def test_deletes_commits_and_files_only(
        self,
        session_factory: Callable[[], AsyncSession],
        repo_with_commits: tuple[GitRepo, list[GitCommit]],
    ) -> None:
        """Test that delete_by_repo_id only deletes commits and files, not repos."""
        commit_repository = create_git_commit_repository(session_factory)
        repo, commits = repo_with_commits

        # Verify initial state
        async with session_factory() as session:
            initial_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            initial_files = await session.scalar(
                text("SELECT COUNT(*) FROM git_commit_files")
            )
            initial_repos = await session.scalar(text("SELECT COUNT(*) FROM git_repos"))

            assert initial_commits == 2
            assert initial_files == 1  # Only first commit has files
            assert initial_repos == 1

        # Delete commits
        assert repo.id is not None
        await commit_repository.delete_by_repo_id(repo.id)

        # Verify only commits and their files were deleted
        async with session_factory() as session:
            remaining_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            remaining_files = await session.scalar(
                text("SELECT COUNT(*) FROM git_commit_files")
            )
            remaining_repos = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos")
            )

            assert remaining_commits == 0
            assert remaining_files == 0
            assert remaining_repos == 1    # Repos should remain

    async def test_handles_nonexistent_repo(
        self,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that deleting commits for non-existent repo handles gracefully."""
        commit_repository = create_git_commit_repository(session_factory)

        # Should not raise an exception
        await commit_repository.delete_by_repo_id(99999)


async def test_save_and_get_commits(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
    sample_git_commit: GitCommit,
) -> None:
    """Test saving and retrieving commits."""
    commit_repository = create_git_commit_repository(session_factory)
    repo_repository = create_git_repo_repository(session_factory)

    # Save repository
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None

    # Save commit
    await commit_repository.save_bulk([sample_git_commit], saved_repo.id)

    # Retrieve commit
    retrieved_commits = await commit_repository.get_by_repo_id(saved_repo.id)
    assert len(retrieved_commits) == 1
    assert retrieved_commits[0].commit_sha == sample_git_commit.commit_sha
    assert retrieved_commits[0].message == sample_git_commit.message

    # Test get by SHA
    retrieved_commit = await commit_repository.get_by_sha(sample_git_commit.commit_sha)
    assert retrieved_commit is not None
    assert retrieved_commit.commit_sha == sample_git_commit.commit_sha


async def test_save_multiple_commits(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
    sample_git_file: GitFile,
) -> None:
    """Test saving multiple commits for a repository."""
    commit_repository = create_git_commit_repository(session_factory)
    repo_repository = create_git_repo_repository(session_factory)

    # Save repository
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None

    # Create multiple commits
    commits = [
        GitCommit(
            created_at=datetime.now(UTC),
            commit_sha="commit1",
            date=datetime.now(UTC),
            message="First commit",
            parent_commit_sha=None,
            files=[sample_git_file],
            author="author1@example.com",
        ),
        GitCommit(
            created_at=datetime.now(UTC),
            commit_sha="commit2",
            date=datetime.now(UTC),
            message="Second commit",
            parent_commit_sha="commit1",
            files=[],
            author="author2@example.com",
        ),
    ]

    # Save all commits
    await commit_repository.save_bulk(commits, saved_repo.id)

    # Retrieve and verify
    retrieved_commits = await commit_repository.get_by_repo_id(saved_repo.id)
    assert len(retrieved_commits) == 2
    commit_shas = {commit.commit_sha for commit in retrieved_commits}
    assert commit_shas == {"commit1", "commit2"}


async def test_empty_repository_returns_empty_list(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
) -> None:
    """Test querying commits for a repository with no commits returns empty list."""
    commit_repository = create_git_commit_repository(session_factory)
    repo_repository = create_git_repo_repository(session_factory)

    # Save repository without commits
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None

    # Query commits for the empty repository
    commits = await commit_repository.get_by_repo_id(saved_repo.id)
    assert commits == []


async def test_nonexistent_commit_raises_error(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that querying for a non-existent commit raises ValueError."""
    commit_repository = create_git_commit_repository(session_factory)

    # Query for a commit that doesn't exist - should raise ValueError
    with pytest.raises(ValueError, match="not found"):
        await commit_repository.get_by_sha("nonexistent_sha")
