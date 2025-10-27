"""Tests for SqlAlchemyGitCommitRepository."""

from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitFile, GitRepo
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    SqlAlchemyGitCommitRepository,
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
            repo_id=saved_repo.id,
            date=datetime.now(UTC),
            message="First commit",
            parent_commit_sha=None,
            files=[sample_git_file],
            author="test@example.com",
        ),
        GitCommit(
            created_at=datetime.now(UTC),
            commit_sha="commit2",
            repo_id=saved_repo.id,
            date=datetime.now(UTC),
            message="Second commit",
            parent_commit_sha="commit1",
            files=[],
            author="test@example.com",
        ),
    ]

    await commit_repository.save_bulk(commits)
    return saved_repo, commits


class TestCommitDeletion:
    """Test commit deletion functionality."""

    async def test_deletes_commits_and_files_only(
        self,
        session_factory: Callable[[], AsyncSession],
        repo_with_commits: tuple[GitRepo, list[GitCommit]],
    ) -> None:
        """Test that delete_by_repo_id deletes commits and files."""
        commit_repository = create_git_commit_repository(session_factory)
        repo_repository = create_git_repo_repository(session_factory)
        repo, commits = repo_with_commits

        # Verify commits exist
        assert repo.id is not None
        initial_count = await commit_repository.count_by_repo_id(repo.id)
        assert initial_count == 2

        # Delete commits
        await commit_repository.delete_by_repo_id(repo.id)

        # Verify commits are deleted
        remaining_count = await commit_repository.count_by_repo_id(repo.id)
        assert remaining_count == 0

        # Verify repo still exists
        retrieved_repo = await repo_repository.get_by_id(repo.id)
        assert retrieved_repo is not None


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
    await commit_repository.save_bulk([sample_git_commit])

    # Retrieve commit
    retrieved_commits = await commit_repository.get_by_repo_id(saved_repo.id)
    assert len(retrieved_commits) == 1
    assert retrieved_commits[0].commit_sha == sample_git_commit.commit_sha
    assert retrieved_commits[0].message == sample_git_commit.message

    # Test get by SHA
    retrieved_commit = await commit_repository.get(sample_git_commit.commit_sha)
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
            repo_id=saved_repo.id,
            date=datetime.now(UTC),
            message="First commit",
            parent_commit_sha=None,
            files=[sample_git_file],
            author="author1@example.com",
        ),
        GitCommit(
            created_at=datetime.now(UTC),
            commit_sha="commit2",
            repo_id=saved_repo.id,
            date=datetime.now(UTC),
            message="Second commit",
            parent_commit_sha="commit1",
            files=[],
            author="author2@example.com",
        ),
    ]

    # Save all commits
    await commit_repository.save_bulk(commits)

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


@pytest.fixture
def repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemyGitCommitRepository:
    """Create a repository with a session factory."""
    return SqlAlchemyGitCommitRepository(session_factory)


def create_commits_with_files(
    num_commits: int, files_per_commit: int = 10
) -> list[GitCommit]:
    """Create a list of commits for testing bulk operations."""
    commits = []
    for i in range(num_commits):
        files = [
            GitFile(
                commit_sha=f"commit_sha_{i:06d}",
                created_at=datetime.now(UTC),
                blob_sha=f"file_sha_{i}_{j}",
                path=f"src/file_{i}_{j}.py",
                mime_type="text/x-python",
                size=1024 + j,
                extension="py",
            )
            for j in range(files_per_commit)
        ]

        commit = GitCommit(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            commit_sha=f"commit_sha_{i:06d}",
            repo_id=1,
            date=datetime.now(UTC),
            message=f"Commit {i}",
            parent_commit_sha=f"parent_sha_{i - 1:06d}" if i > 0 else None,
            files=files,
            author=f"Author {i}",
        )
        commits.append(commit)
    return commits


@pytest.fixture
async def test_repo_with_id(
    session_factory: Callable[[], AsyncSession],
) -> int:
    """Create a test repository and return its ID."""
    from kodit.infrastructure.sqlalchemy import entities as db_entities
    from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

    async with SqlAlchemyUnitOfWork(session_factory) as session:
        test_repo = db_entities.GitRepo(
            sanitized_remote_uri="https://github.com/test/repo",
            remote_uri="https://github.com/test/repo.git",
            cloned_path=Path("/tmp/test/repo"),
            num_commits=10,
            num_branches=1,
            num_tags=0,
        )
        session.add(test_repo)
        await session.flush()
        return test_repo.id


class TestSaveBulk:
    """Test save_bulk() method with normal operations."""

    async def test_saves_new_commits_in_bulk(
        self,
        repository: SqlAlchemyGitCommitRepository,
        test_repo_with_id: int,  # noqa: ARG002
    ) -> None:
        """Test that save_bulk() creates new commits."""
        commits = create_commits_with_files(10)
        await repository.save_bulk(commits)

        # Verify commits were saved
        for commit in commits:
            assert await repository.exists(commit.commit_sha)

    async def test_skips_existing_commits_in_bulk(
        self,
        repository: SqlAlchemyGitCommitRepository,
        test_repo_with_id: int,  # noqa: ARG002
    ) -> None:
        """Test that save_bulk() skips existing commits."""
        commits = create_commits_with_files(5)

        # Save twice
        await repository.save_bulk(commits)
        await repository.save_bulk(commits)

        # Verify commits exist
        for commit in commits:
            assert await repository.exists(commit.commit_sha)

    async def test_handles_empty_commit_list(
        self,
        repository: SqlAlchemyGitCommitRepository,
        test_repo_with_id: int,  # noqa: ARG002
    ) -> None:
        """Test that save_bulk() handles empty commit lists."""
        await repository.save_bulk([])

    async def test_saves_commits_with_no_files(
        self,
        repository: SqlAlchemyGitCommitRepository,
        test_repo_with_id: int,  # noqa: ARG002
    ) -> None:
        """Test that save_bulk() handles commits with no files."""
        commits = create_commits_with_files(5, files_per_commit=0)
        await repository.save_bulk(commits)

        # Verify commits were saved
        for commit in commits:
            assert await repository.exists(commit.commit_sha)
