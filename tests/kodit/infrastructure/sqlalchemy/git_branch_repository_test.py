"""Tests for SqlAlchemyGitBranchRepository."""

from collections.abc import Callable
from datetime import UTC, datetime

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitBranch, GitCommit, GitRepo
from kodit.infrastructure.sqlalchemy.git_branch_repository import (
    create_git_branch_repository,
)
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository


@pytest.fixture
async def repo_with_branches(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
    sample_git_commit: GitCommit,
) -> tuple[GitRepo, list[GitBranch]]:
    """Create a repository with branches for testing."""
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)
    branch_repository = create_git_branch_repository(session_factory)

    # Save repository and commit
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None
    await commit_repository.save_bulk([sample_git_commit])

    # Create branches
    branches = [
        GitBranch(
            created_at=datetime.now(UTC),
            repo_id=saved_repo.id,
            name="main",
            head_commit_sha=sample_git_commit.commit_sha,
        ),
        GitBranch(
            created_at=datetime.now(UTC),
            repo_id=saved_repo.id,
            name="develop",
            head_commit_sha=sample_git_commit.commit_sha,
        ),
    ]

    await branch_repository.save_bulk(branches)
    return saved_repo, branches


class TestBranchDeletion:
    """Test branch deletion functionality."""

    async def test_deletes_branches_only(
        self,
        session_factory: Callable[[], AsyncSession],
        repo_with_branches: tuple[GitRepo, list[GitBranch]],
    ) -> None:
        """Test that delete_by_repo_id only deletes branches, not other entities."""
        branch_repository = create_git_branch_repository(session_factory)
        repo, branches = repo_with_branches

        # Verify initial state
        async with session_factory() as session:
            initial_branches = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches")
            )
            initial_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            initial_repos = await session.scalar(text("SELECT COUNT(*) FROM git_repos"))

            assert initial_branches == 2
            assert initial_commits == 1
            assert initial_repos == 1

        # Delete branches
        assert repo.id is not None
        await branch_repository.delete_by_repo_id(repo.id)

        # Verify only branches were deleted
        async with session_factory() as session:
            remaining_branches = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches")
            )
            remaining_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            remaining_repos = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos")
            )

            assert remaining_branches == 0
            assert remaining_commits == 1  # Commits should remain
            assert remaining_repos == 1  # Repos should remain

    async def test_handles_nonexistent_repo(
        self,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that deleting branches for non-existent repo handles gracefully."""
        branch_repository = create_git_branch_repository(session_factory)

        # Should not raise an exception
        await branch_repository.delete_by_repo_id(99999)


async def test_save_and_get_branches(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
    sample_git_commit: GitCommit,
) -> None:
    """Test saving and retrieving branches."""
    branch_repository = create_git_branch_repository(session_factory)
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)

    # Save repository and commit
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None
    await commit_repository.save_bulk([sample_git_commit])

    # Create and save branch
    branch = GitBranch(
        repo_id=saved_repo.id,
        name="main",
        head_commit_sha=sample_git_commit.commit_sha,
    )
    await branch_repository.save(branch)

    # Retrieve branches
    retrieved_branches = await branch_repository.get_by_repo_id(saved_repo.id)
    assert len(retrieved_branches) == 1
    assert retrieved_branches[0].name == "main"
    assert retrieved_branches[0].repo_id == saved_repo.id


async def test_save_multiple_branches(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
    sample_git_commit: GitCommit,
) -> None:
    """Test saving multiple branches for a repository."""
    branch_repository = create_git_branch_repository(session_factory)
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)

    # Save repository and commit
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None
    await commit_repository.save_bulk([sample_git_commit])

    # Create multiple branches
    branches = [
        GitBranch(
            created_at=datetime.now(UTC),
            repo_id=saved_repo.id,
            name="main",
            head_commit_sha=sample_git_commit.commit_sha,
        ),
        GitBranch(
            created_at=datetime.now(UTC),
            repo_id=saved_repo.id,
            name="develop",
            head_commit_sha=sample_git_commit.commit_sha,
        ),
    ]

    # Save all branches
    await branch_repository.save_bulk(branches)

    # Retrieve and verify
    retrieved_branches = await branch_repository.get_by_repo_id(saved_repo.id)
    assert len(retrieved_branches) == 2
    branch_names = {branch.name for branch in retrieved_branches}
    assert branch_names == {"main", "develop"}


async def test_empty_repository_returns_empty_list(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
) -> None:
    """Test querying branches for a repository with no branches returns empty list."""
    repo_repository = create_git_repo_repository(session_factory)
    branch_repository = create_git_branch_repository(session_factory)

    # Save repository without branches
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None

    # Query branches for the empty repository
    branches = await branch_repository.get_by_repo_id(saved_repo.id)
    assert branches == []
