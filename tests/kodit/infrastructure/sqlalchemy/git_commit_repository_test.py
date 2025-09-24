"""Tests for SqlAlchemyGitCommitRepository."""

from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitFile
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    SqlAlchemyGitCommitRepository,
)


@pytest.fixture
def repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemyGitCommitRepository:
    """Create a repository with a session factory."""
    return SqlAlchemyGitCommitRepository(session_factory)


@pytest.fixture
def sample_git_file() -> GitFile:
    """Create a sample git file."""
    return GitFile(
        created_at=datetime.now(UTC),
        blob_sha="file_sha_123",
        path="src/main.py",
        mime_type="text/x-python",
        size=1024,
        extension="py",
    )


@pytest.fixture
def sample_git_commit(sample_git_file: GitFile) -> GitCommit:
    """Create a sample git commit."""
    return GitCommit(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        commit_sha="commit_sha_456",
        date=datetime.now(UTC),
        message="Initial commit",
        parent_commit_sha="parent_sha_789",
        files=[sample_git_file],
        author="Test Author",
    )


def create_large_commit_list(num_commits: int) -> list[GitCommit]:
    """Create a large list of commits for testing bulk operations."""
    commits = []
    for i in range(num_commits):
        # Create multiple files per commit to increase parameter count
        files = []
        for j in range(10):  # 10 files per commit
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
            parent_commit_sha=f"parent_sha_{i-1:06d}" if i > 0 else None,
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
        """Test to demonstrate the PostgreSQL parameter limit issue.

        This test documents the parameter limit issue that occurs in production.
        In PostgreSQL, the limit is 32767 parameters per query.
        For commits: 32767 / 8 fields = ~4095 commits maximum.

        Note: This test may pass in SQLite (test env) but would fail in PostgreSQL.
        """
        # Create a test repository first to avoid foreign key errors
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
                num_tags=0
            )
            session.add(test_repo)
            await session.flush()
            repo_id = test_repo.id

        # Create 5000 commits that would exceed PostgreSQL's 32767 parameter limit
        # 5000 commits * 8 fields = 40,000 parameters (exceeds PostgreSQL limit)
        large_commits = []
        for i in range(5000):
            commit = GitCommit(
                created_at=datetime.now(UTC),
                updated_at=datetime.now(UTC),
                commit_sha=f"commit_sha_{i:06d}",
                date=datetime.now(UTC),
                message=f"Commit {i}",
                parent_commit_sha=f"parent_sha_{i-1:06d}" if i > 0 else None,
                files=[],  # NO FILES to focus on commits only
                author=f"Author {i}",
            )
            large_commits.append(commit)

        # In PostgreSQL production, this would fail with parameter limit error
        # In SQLite test env, this may pass - but the fix should handle both
        try:
            await repository.save_bulk(large_commits, repo_id)
            # If no error, verify all commits were saved (test passed with chunking)
            for i in range(0, min(100, len(large_commits)), 10):  # Sample check
                assert await repository.exists(large_commits[i].commit_sha)
        except Exception as exc:
            # If error occurs, check if it's the parameter limit error
            error_msg = str(exc).lower()
            is_parameter_error = any(phrase in error_msg for phrase in [
                "the number of query arguments cannot exceed 32767",  # PostgreSQL
                "too many sql variables",  # SQLite
                "parameter limit",
                "too many parameters",
                "variable number limit exceeded",
                "bind variables"
            ])

            if is_parameter_error:
                pytest.fail(
                    f"Parameter limit error occurred - chunking fix needed: {exc}"
                )
            else:
                # Re-raise non-parameter-limit errors
                raise

    @pytest.mark.asyncio
    async def test_bulk_save_with_large_file_count_exceeds_limit(
        self,
        repository: SqlAlchemyGitCommitRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test bulk save where files cause parameter limit to be exceeded."""
        # Create a test repository first to avoid foreign key errors
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
                num_tags=0
            )
            session.add(test_repo)
            await session.flush()
            repo_id = test_repo.id

        # Each GitCommitFile has 7 fields for commit_sha, path, blob_sha, etc.
        # With 32767 parameters, that's ~4681 files maximum
        # Let's create fewer commits but more files per commit
        commits = []

        # Create 100 commits with 50 files each = 5000 files
        # This should exceed the parameter limit when inserting files
        for i in range(100):
            files = []
            for j in range(50):  # 50 files per commit
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
                parent_commit_sha=f"parent_sha_{i-1:06d}" if i > 0 else None,
                files=files,
                author=f"Author {i}",
            )
            commits.append(commit)

        # In PostgreSQL production, this would fail with parameter limit error
        # In SQLite test env, this may pass - but the fix should handle both
        try:
            await repository.save_bulk(commits, repo_id)
            # If no error, verify some commits were saved (test passed with chunking)
            for i in range(min(10, len(commits))):  # Sample check
                assert await repository.exists(commits[i].commit_sha)
        except Exception as exc:
            # If error occurs, check if it's the parameter limit error
            error_msg = str(exc).lower()
            is_parameter_error = any(phrase in error_msg for phrase in [
                "the number of query arguments cannot exceed 32767",  # PostgreSQL
                "too many sql variables",  # SQLite
                "parameter limit",
                "too many parameters",
                "variable number limit exceeded",
                "bind variables"
            ])

            if is_parameter_error:
                pytest.fail(
                    f"Parameter limit error occurred - chunking fix needed: {exc}"
                )
            else:
                # Re-raise non-parameter-limit errors
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
        # Create a test repository first
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
                num_tags=0
            )
            session.add(test_repo)
            await session.flush()
            repo_id = test_repo.id

        commits = create_large_commit_list(10)  # Small number for normal test

        await repository.save_bulk(commits, repo_id)

        # Verify commits were saved
        for commit in commits:
            assert await repository.exists(commit.commit_sha)

    @pytest.mark.asyncio
    async def test_skips_existing_commits_in_bulk(
        self,
        repository: SqlAlchemyGitCommitRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that save_bulk() skips existing commits."""
        # Create a test repository first
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
                num_tags=0
            )
            session.add(test_repo)
            await session.flush()
            repo_id = test_repo.id

        commits = create_large_commit_list(5)

        # Save once
        await repository.save_bulk(commits, repo_id)

        # Save again - should not error and should skip existing
        await repository.save_bulk(commits, repo_id)

        # All should still exist
        for commit in commits:
            assert await repository.exists(commit.commit_sha)

    @pytest.mark.asyncio
    async def test_handles_empty_commit_list(
        self,
        repository: SqlAlchemyGitCommitRepository,
    ) -> None:
        """Test that save_bulk() handles empty commit lists."""
        await repository.save_bulk([], 1)  # Should not error

    @pytest.mark.asyncio
    async def test_saves_commits_with_no_files(
        self,
        repository: SqlAlchemyGitCommitRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that save_bulk() handles commits with no files."""
        # Create a test repository first
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
                num_tags=0
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
                files=[],  # No files
                author=f"Author {i}",
            )
            commits.append(commit)

        await repository.save_bulk(commits, repo_id)

        # Verify commits were saved
        for commit in commits:
            assert await repository.exists(commit.commit_sha)
