"""Tests for SqlAlchemyGitCommitRepository."""

from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import pytest
from sqlalchemy import text
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
        repo, _ = repo_with_commits

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
            assert remaining_repos == 1  # Repos should remain

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


@pytest.fixture
def repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemyGitCommitRepository:
    """Create a repository with a session factory."""
    return SqlAlchemyGitCommitRepository(session_factory)


@pytest.fixture
def local_sample_git_file() -> GitFile:
    """Create a sample git file for bulk tests in this module."""
    return GitFile(
        created_at=datetime.now(UTC),
        blob_sha="file_sha_123",
        path="src/main.py",
        mime_type="text/x-python",
        size=1024,
        extension="py",
    )


@pytest.fixture
def local_sample_git_commit(local_sample_git_file: GitFile) -> GitCommit:
    """Create a sample git commit for bulk tests in this module."""
    return GitCommit(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        commit_sha="commit_sha_456",
        date=datetime.now(UTC),
        message="Initial commit",
        parent_commit_sha="parent_sha_789",
        files=[local_sample_git_file],
        author="Test Author",
    )


def create_large_commit_list(num_commits: int) -> list[GitCommit]:
    """Create a large list of commits for testing bulk operations."""
    commits = []
    for i in range(num_commits):
        files = []
        for j in range(10):
            file = GitFile(
                created_at=datetime.now(UTC),
                blob_sha=f"file_sha_{i}_{j}",
                path=f"src/file_{i}_{j}.py",
                mime_type="text/x-python",
                size=1024 + j,
                extension="py",
            )
            files.append(file)

        commit = GitCommit(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            commit_sha=f"commit_sha_{i:06d}",
            date=datetime.now(UTC),
            message=f"Commit {i}",
            parent_commit_sha=f"parent_sha_{i - 1:06d}" if i > 0 else None,
            files=files,
            author=f"Author {i}",
        )
        commits.append(commit)
    return commits


class TestBulkInsertLimits:
    """Test bulk insert operations with large data sets."""

    @pytest.mark.asyncio
    async def test_bulk_save_demonstrates_parameter_limit_issue(
        self,
        repository: SqlAlchemyGitCommitRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test to demonstrate the PostgreSQL parameter limit issue."""
        from kodit.infrastructure.sqlalchemy import entities as db_entities
        from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

        repo_id = None
        async with SqlAlchemyUnitOfWork(session_factory) as session:
            test_repo = db_entities.GitRepo(
                sanitized_remote_uri="https://github.com/test/large-repo",
                remote_uri="https://github.com/test/large-repo.git",
                cloned_path=Path("/tmp/test/large-repo"),
                num_commits=5000,
                num_branches=1,
                num_tags=0,
            )
            session.add(test_repo)
            await session.flush()
            repo_id = test_repo.id

        large_commits = []
        for i in range(5000):
            commit = GitCommit(
                created_at=datetime.now(UTC),
                updated_at=datetime.now(UTC),
                commit_sha=f"commit_sha_{i:06d}",
                date=datetime.now(UTC),
                message=f"Commit {i}",
                parent_commit_sha=f"parent_sha_{i - 1:06d}" if i > 0 else None,
                files=[],
                author=f"Author {i}",
            )
            large_commits.append(commit)

        try:
            await repository.save_bulk(large_commits, repo_id)
            for i in range(0, min(100, len(large_commits)), 10):
                assert await repository.exists(large_commits[i].commit_sha)
        except Exception as exc:
            error_msg = str(exc).lower()
            is_parameter_error = any(
                phrase in error_msg
                for phrase in [
                    "the number of query arguments cannot exceed 32767",
                    "too many sql variables",
                    "parameter limit",
                    "too many parameters",
                    "variable number limit exceeded",
                    "bind variables",
                ]
            )

            if is_parameter_error:
                pytest.fail(
                    f"Parameter limit error occurred - chunking fix needed: {exc}"
                )
            else:
                raise

    @pytest.mark.asyncio
    async def test_bulk_save_with_large_file_count_exceeds_limit(
        self,
        repository: SqlAlchemyGitCommitRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test bulk save where files cause parameter limit to be exceeded."""
        from kodit.infrastructure.sqlalchemy import entities as db_entities
        from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

        repo_id = None
        async with SqlAlchemyUnitOfWork(session_factory) as session:
            test_repo = db_entities.GitRepo(
                sanitized_remote_uri="https://github.com/test/file-heavy-repo",
                remote_uri="https://github.com/test/file-heavy-repo.git",
                cloned_path=Path("/tmp/test/file-heavy-repo"),
                num_commits=100,
                num_branches=1,
                num_tags=0,
            )
            session.add(test_repo)
            await session.flush()
            repo_id = test_repo.id

        commits = []
        for i in range(100):
            files = []
            for j in range(50):
                file = GitFile(
                    created_at=datetime.now(UTC),
                    blob_sha=f"file_sha_{i}_{j}",
                    path=f"src/very_long_path_name_that_takes_more_space_{i}_{j}.py",
                    mime_type="text/x-python",
                    size=1024 + j,
                    extension="py",
                )
                files.append(file)

            commit = GitCommit(
                created_at=datetime.now(UTC),
                updated_at=datetime.now(UTC),
                commit_sha=f"commit_sha_{i:06d}",
                date=datetime.now(UTC),
                message=f"Commit {i}",
                parent_commit_sha=f"parent_sha_{i - 1:06d}" if i > 0 else None,
                files=files,
                author=f"Author {i}",
            )
            commits.append(commit)

        try:
            await repository.save_bulk(commits, repo_id)
            for i in range(min(10, len(commits))):
                assert await repository.exists(commits[i].commit_sha)
        except Exception as exc:
            error_msg = str(exc).lower()
            is_parameter_error = any(
                phrase in error_msg
                for phrase in [
                    "the number of query arguments cannot exceed 32767",
                    "too many sql variables",
                    "parameter limit",
                    "too many parameters",
                    "variable number limit exceeded",
                    "bind variables",
                ]
            )

            if is_parameter_error:
                pytest.fail(
                    f"Parameter limit error occurred - chunking fix needed: {exc}"
                )
            else:
                raise


class TestSaveBulk:
    """Test save_bulk() method with normal operations."""

    @pytest.mark.asyncio
    async def test_saves_new_commits_in_bulk(
        self,
        repository: SqlAlchemyGitCommitRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that save_bulk() creates new commits."""
        from kodit.infrastructure.sqlalchemy import entities as db_entities
        from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

        repo_id = None
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
            repo_id = test_repo.id

        commits = create_large_commit_list(10)

        await repository.save_bulk(commits, repo_id)

        for commit in commits:
            assert await repository.exists(commit.commit_sha)

    @pytest.mark.asyncio
    async def test_skips_existing_commits_in_bulk(
        self,
        repository: SqlAlchemyGitCommitRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that save_bulk() skips existing commits."""
        from kodit.infrastructure.sqlalchemy import entities as db_entities
        from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

        repo_id = None
        async with SqlAlchemyUnitOfWork(session_factory) as session:
            test_repo = db_entities.GitRepo(
                sanitized_remote_uri="https://github.com/test/repo2",
                remote_uri="https://github.com/test/repo2.git",
                cloned_path=Path("/tmp/test/repo2"),
                num_commits=5,
                num_branches=1,
                num_tags=0,
            )
            session.add(test_repo)
            await session.flush()
            repo_id = test_repo.id

        commits = create_large_commit_list(5)

        await repository.save_bulk(commits, repo_id)
        await repository.save_bulk(commits, repo_id)

        for commit in commits:
            assert await repository.exists(commit.commit_sha)

    @pytest.mark.asyncio
    async def test_handles_empty_commit_list(
        self,
        repository: SqlAlchemyGitCommitRepository,
    ) -> None:
        """Test that save_bulk() handles empty commit lists."""
        await repository.save_bulk([], 1)

    @pytest.mark.asyncio
    async def test_saves_commits_with_no_files(
        self,
        repository: SqlAlchemyGitCommitRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that save_bulk() handles commits with no files."""
        from kodit.infrastructure.sqlalchemy import entities as db_entities
        from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork

        repo_id = None
        async with SqlAlchemyUnitOfWork(session_factory) as session:
            test_repo = db_entities.GitRepo(
                sanitized_remote_uri="https://github.com/test/repo3",
                remote_uri="https://github.com/test/repo3.git",
                cloned_path=Path("/tmp/test/repo3"),
                num_commits=5,
                num_branches=1,
                num_tags=0,
            )
            session.add(test_repo)
            await session.flush()
            repo_id = test_repo.id

        commits = []
        for i in range(5):
            commit = GitCommit(
                created_at=datetime.now(UTC),
                updated_at=datetime.now(UTC),
                commit_sha=f"commit_sha_{i:06d}",
                date=datetime.now(UTC),
                message=f"Commit {i}",
                parent_commit_sha=None,
                files=[],
                author=f"Author {i}",
            )
            commits.append(commit)

        await repository.save_bulk(commits, repo_id)

        for commit in commits:
            assert await repository.exists(commit.commit_sha)
