"""Tests for SqlAlchemyGitBranchRepository."""

from collections.abc import Callable
from datetime import UTC, datetime

import pytest
from pydantic import AnyUrl
from sqlalchemy import bindparam, text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitBranch, GitCommit, GitFile, GitRepo
from kodit.infrastructure.sqlalchemy.git_branch_repository import (
    SqlAlchemyGitBranchRepository,
    create_git_branch_repository,
)
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository


@pytest.fixture
def branch_repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemyGitBranchRepository:
    """Create a branch repository with a session factory."""
    return SqlAlchemyGitBranchRepository(session_factory)


@pytest.fixture
def sample_commit() -> GitCommit:
    """Create a sample git commit."""
    sample_file = GitFile(
        created_at=datetime.now(UTC),
        blob_sha="file_blob_123",
        path="src/main.py",
        mime_type="text/x-python",
        size=1024,
        extension="py",
    )

    return GitCommit(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        commit_sha="commit_abc123",
        date=datetime.now(UTC),
        message="Initial commit",
        parent_commit_sha=None,
        files=[sample_file],
        author="Test Author",
    )


@pytest.fixture
def sample_branch(sample_commit: GitCommit) -> GitBranch:
    """Create a sample git branch."""
    return GitBranch(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        repo_id=1,
        name="main",
        head_commit=sample_commit,
    )


@pytest.fixture
async def two_repos_with_branches(
    session_factory: Callable[[], AsyncSession],
    sample_commit: GitCommit,
) -> tuple[GitRepo, GitRepo, list[GitBranch], list[GitBranch]]:
    """Create two repositories with multiple branches each."""
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)
    branch_repository = create_git_branch_repository(session_factory)

    # Create two repositories
    repo1 = GitRepo(
        id=None,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo1"),
        remote_uri=AnyUrl("https://github.com/test/repo1.git"),
        tracking_branch=None,
        num_commits=1,
        num_branches=3,
        num_tags=0,
    )
    repo1 = await repo_repository.save(repo1)

    repo2 = GitRepo(
        id=None,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo2"),
        remote_uri=AnyUrl("https://github.com/test/repo2.git"),
        tracking_branch=None,
        num_commits=1,
        num_branches=2,
        num_tags=0,
    )
    repo2 = await repo_repository.save(repo2)

    assert repo1.id is not None
    assert repo2.id is not None

    # Save commits to both repos
    await commit_repository.save_bulk([sample_commit], repo1.id)

    # Create a different commit for repo2
    commit2 = GitCommit(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        commit_sha="commit_def456",
        date=datetime.now(UTC),
        message="Second commit",
        parent_commit_sha=None,
        files=[],
        author="Test Author 2",
    )
    await commit_repository.save_bulk([commit2], repo2.id)

    # Create branches for repo1
    branches1 = [
        GitBranch(
            created_at=datetime.now(UTC),
            repo_id=repo1.id,
            name="main",
            head_commit=sample_commit,
        ),
        GitBranch(
            created_at=datetime.now(UTC),
            repo_id=repo1.id,
            name="develop",
            head_commit=sample_commit,
        ),
        GitBranch(
            created_at=datetime.now(UTC),
            repo_id=repo1.id,
            name="feature/awesome",
            head_commit=sample_commit,
        ),
    ]

    # Create branches for repo2
    branches2 = [
        GitBranch(
            created_at=datetime.now(UTC),
            repo_id=repo2.id,
            name="main",
            head_commit=commit2,
        ),
        GitBranch(
            created_at=datetime.now(UTC),
            repo_id=repo2.id,
            name="hotfix",
            head_commit=commit2,
        ),
    ]

    # Save all branches
    await branch_repository.save_bulk(branches1, repo1.id)
    await branch_repository.save_bulk(branches2, repo2.id)

    return repo1, repo2, branches1, branches2


class TestDeleteByRepoId:
    """Test delete_by_repo_id() method for DDD compliance."""

    async def test_only_deletes_branches_not_commits_or_repos(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        two_repos_with_branches: tuple[
            GitRepo, GitRepo, list[GitBranch], list[GitBranch]
        ],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete_by_repo_id() only deletes branches, not commits or repos.

        This test verifies DDD compliance: the branch repository should only delete
        the entities it directly controls (GitBranch), leaving commits and repos
        to be managed by their respective repositories.
        """
        repo1, repo2, branches1, branches2 = two_repos_with_branches

        # Verify initial state exists
        async with session_factory() as session:
            # Check both repos exist
            repo_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos")
            )
            assert repo_count == 2

            # Check commits exist
            commit_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            assert commit_count == 2  # One commit per repo

            # Check branches exist (3 for repo1 + 2 for repo2 = 5 total)
            branch_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches")
            )
            assert branch_count == 5

        # Delete branches for repo1 only
        assert repo1.id is not None
        await branch_repository.delete_by_repo_id(repo1.id)

        # Verify only repo1's branches were deleted, not repos or commits
        async with session_factory() as session:
            # Both repos should still exist (branch repository doesn't control repos)
            repo_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos")
            )
            assert repo_count == 2, "Repositories should not be deleted"

            # Both commits should still exist (branch repo doesn't control commits)
            commit_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            assert commit_count == 2, "Commits should not be deleted"

            # Only repo1's branches should be deleted
            remaining_branches = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches")
            )
            assert remaining_branches == 2, "Only repo2's branches should remain"

            # Verify it's repo2's branches that remain
            repo2_branches = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches WHERE repo_id = :repo_id"),
                {"repo_id": repo2.id}
            )
            assert repo2_branches == 2, "All repo2's branches should remain"

    async def test_deletes_only_branches_for_specified_repo(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        two_repos_with_branches: tuple[
            GitRepo, GitRepo, list[GitBranch], list[GitBranch]
        ],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete_by_repo_id() only affects the specified repository."""
        repo1, repo2, branches1, branches2 = two_repos_with_branches

        # Verify initial branch counts
        async with session_factory() as session:
            repo1_branch_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches WHERE repo_id = :repo_id"),
                {"repo_id": repo1.id}
            )
            repo2_branch_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches WHERE repo_id = :repo_id"),
                {"repo_id": repo2.id}
            )
            assert repo1_branch_count == 3
            assert repo2_branch_count == 2

        # Delete branches for repo2 only
        assert repo2.id is not None
        await branch_repository.delete_by_repo_id(repo2.id)

        # Verify only repo2's branches were affected
        async with session_factory() as session:
            # Repo1's branches should remain untouched
            repo1_branches = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches WHERE repo_id = :repo_id"),
                {"repo_id": repo1.id}
            )
            assert repo1_branches == 3, "Repo1's branches should remain untouched"

            # Repo2's branches should be gone
            repo2_branches = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches WHERE repo_id = :repo_id"),
                {"repo_id": repo2.id}
            )
            assert repo2_branches == 0, "Repo2's branches should be deleted"

    async def test_deletes_all_branches_for_repo(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        two_repos_with_branches: tuple[
            GitRepo, GitRepo, list[GitBranch], list[GitBranch]
        ],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete_by_repo_id() deletes all branches for a repository."""
        repo1, repo2, branches1, branches2 = two_repos_with_branches

        # Verify repo1 has multiple branches with different names
        async with session_factory() as session:
            branch_names = await session.execute(
                text(
                    "SELECT name FROM git_branches "
                    "WHERE repo_id = :repo_id ORDER BY name"
                ),
                {"repo_id": repo1.id}
            )
            names = [row[0] for row in branch_names]
            expected_names = ["develop", "feature/awesome", "main"]
            assert names == expected_names, "Should have all expected branch names"

        # Delete all branches for repo1
        assert repo1.id is not None
        await branch_repository.delete_by_repo_id(repo1.id)

        # Verify all branches were deleted
        async with session_factory() as session:
            remaining_repo1_branches = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches WHERE repo_id = :repo_id"),
                {"repo_id": repo1.id}
            )
            assert remaining_repo1_branches == 0, (
                "All repo1's branches should be deleted"
            )

            # Verify repo2's branches are unaffected
            repo2_branches = await session.scalar(
                text("SELECT COUNT(*) FROM git_branches WHERE repo_id = :repo_id"),
                {"repo_id": repo2.id}
            )
            assert repo2_branches == 2, "Repo2's branches should be unaffected"

    async def test_handles_nonexistent_repo_gracefully(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
    ) -> None:
        """Test that delete_by_repo_id() handles non-existent repo IDs gracefully."""
        # Should not raise an exception when deleting branches for non-existent repo
        await branch_repository.delete_by_repo_id(99999)

    async def test_handles_repo_with_no_branches_gracefully(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete_by_repo_id() handles repos with no branches gracefully."""
        # Create an empty repository
        repo_repository = create_git_repo_repository(session_factory)
        empty_repo = GitRepo(
            id=None,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            sanitized_remote_uri=AnyUrl("https://github.com/test/empty"),
            remote_uri=AnyUrl("https://github.com/test/empty.git"),
            tracking_branch=None,
            num_commits=0,
            num_branches=0,
            num_tags=0,
        )
        empty_repo = await repo_repository.save(empty_repo)
        assert empty_repo.id is not None

        # Should not raise an exception when deleting branches for empty repo
        await branch_repository.delete_by_repo_id(empty_repo.id)

        # Verify repo still exists (wasn't affected by branch deletion)
        async with session_factory() as session:
            repo_exists = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos WHERE id = :repo_id"),
                {"repo_id": empty_repo.id}
            )
            assert repo_exists == 1, "Empty repo should still exist"

    async def test_branch_deletion_doesnt_affect_commits(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        two_repos_with_branches: tuple[
            GitRepo, GitRepo, list[GitBranch], list[GitBranch]
        ],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that deleting branches doesn't affect commits that branches reference.

        This test verifies that the branch repository follows DDD principles by
        not deleting commits even though branches reference them via head_commit.
        """
        repo1, repo2, branches1, branches2 = two_repos_with_branches

        # Get the commit SHAs that branches reference
        async with session_factory() as session:
            referenced_commits = await session.execute(
                text("""
                SELECT DISTINCT c.commit_sha
                FROM git_commits c
                JOIN git_branches b ON c.commit_sha = b.head_commit_sha
                WHERE b.repo_id = :repo_id
                """),
                {"repo_id": repo1.id}
            )
            commit_shas_before = {row[0] for row in referenced_commits}
            assert len(commit_shas_before) > 0, (
                "Should have commits referenced by branches"
            )

        # Delete branches for repo1
        assert repo1.id is not None
        await branch_repository.delete_by_repo_id(repo1.id)

        # Verify commits still exist (branches don't control commits)
        async with session_factory() as session:
            remaining_commits = await session.execute(
                text(
                    "SELECT commit_sha FROM git_commits WHERE commit_sha IN :shas"
                ).bindparams(
                    bindparam("shas", expanding=True)
                ),
                {"shas": list(commit_shas_before)}
            )
            commit_shas_after = {row[0] for row in remaining_commits}
            assert commit_shas_after == commit_shas_before, (
                "Commits should still exist after branch deletion"
            )


class TestCountByRepoId:
    """Test count_by_repo_id() method."""

    async def test_returns_correct_count(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        two_repos_with_branches: tuple[
            GitRepo, GitRepo, list[GitBranch], list[GitBranch]
        ],
    ) -> None:
        """Test that count_by_repo_id() returns the correct count."""
        repo1, repo2, branches1, branches2 = two_repos_with_branches

        assert repo1.id is not None
        assert repo2.id is not None

        # Repo1 should have 3 branches
        count1 = await branch_repository.count_by_repo_id(repo1.id)
        assert count1 == 3

        # Repo2 should have 2 branches
        count2 = await branch_repository.count_by_repo_id(repo2.id)
        assert count2 == 2

    async def test_returns_zero_for_nonexistent_repo(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
    ) -> None:
        """Test that count_by_repo_id() returns 0 for non-existent repo."""
        count = await branch_repository.count_by_repo_id(99999)
        assert count == 0

    async def test_returns_zero_after_deletion(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        two_repos_with_branches: tuple[
            GitRepo, GitRepo, list[GitBranch], list[GitBranch]
        ],
    ) -> None:
        """Test that count_by_repo_id() returns 0 after branches are deleted."""
        repo1, repo2, branches1, branches2 = two_repos_with_branches

        assert repo1.id is not None

        # Initially should have branches
        count_before = await branch_repository.count_by_repo_id(repo1.id)
        assert count_before == 3

        # Delete branches
        await branch_repository.delete_by_repo_id(repo1.id)

        # Count should be zero after deletion
        count_after = await branch_repository.count_by_repo_id(repo1.id)
        assert count_after == 0


class TestExists:
    """Test exists() method."""

    async def test_returns_true_for_existing_branch(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        two_repos_with_branches: tuple[
            GitRepo, GitRepo, list[GitBranch], list[GitBranch]
        ],
    ) -> None:
        """Test that exists() returns True for existing branch."""
        repo1, repo2, branches1, branches2 = two_repos_with_branches

        assert repo1.id is not None
        exists = await branch_repository.exists("main", repo1.id)
        assert exists is True

        exists_develop = await branch_repository.exists("develop", repo1.id)
        assert exists_develop is True

    async def test_returns_false_for_nonexistent_branch(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        two_repos_with_branches: tuple[
            GitRepo, GitRepo, list[GitBranch], list[GitBranch]
        ],
    ) -> None:
        """Test that exists() returns False for non-existent branch."""
        repo1, repo2, branches1, branches2 = two_repos_with_branches

        assert repo1.id is not None
        exists = await branch_repository.exists("nonexistent-branch", repo1.id)
        assert exists is False

    async def test_returns_false_for_nonexistent_repo(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
    ) -> None:
        """Test that exists() returns False for non-existent repo."""
        exists = await branch_repository.exists("main", 99999)
        assert exists is False

    async def test_returns_false_after_deletion(
        self,
        branch_repository: SqlAlchemyGitBranchRepository,
        two_repos_with_branches: tuple[
            GitRepo, GitRepo, list[GitBranch], list[GitBranch]
        ],
    ) -> None:
        """Test that exists() returns False after branches are deleted."""
        repo1, repo2, branches1, branches2 = two_repos_with_branches

        assert repo1.id is not None

        # Initially should exist
        exists_before = await branch_repository.exists("main", repo1.id)
        assert exists_before is True

        # Delete branches
        await branch_repository.delete_by_repo_id(repo1.id)

        # Should not exist after deletion
        exists_after = await branch_repository.exists("main", repo1.id)
        assert exists_after is False
