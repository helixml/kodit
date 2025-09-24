"""Tests for SqlAlchemyGitCommitRepository."""

from collections.abc import Callable
from datetime import UTC, datetime

import pytest
from pydantic import AnyUrl
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitFile, GitRepo
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    SqlAlchemyGitCommitRepository,
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository


@pytest.fixture
def commit_repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemyGitCommitRepository:
    """Create a commit repository with a session factory."""
    return SqlAlchemyGitCommitRepository(session_factory)


@pytest.fixture
def sample_file() -> GitFile:
    """Create a sample git file."""
    return GitFile(
        created_at=datetime.now(UTC),
        blob_sha="shared_file_blob_123",
        path="shared/utils.py",
        mime_type="text/x-python",
        size=2048,
        extension="py",
    )


@pytest.fixture
def unique_file() -> GitFile:
    """Create a unique git file that won't be shared."""
    return GitFile(
        created_at=datetime.now(UTC),
        blob_sha="unique_file_blob_456",
        path="unique/specific.py",
        mime_type="text/x-python",
        size=1024,
        extension="py",
    )


@pytest.fixture
def another_unique_file() -> GitFile:
    """Create another unique git file."""
    return GitFile(
        created_at=datetime.now(UTC),
        blob_sha="another_unique_blob_789",
        path="another/module.py",
        mime_type="text/x-python",
        size=512,
        extension="py",
    )


@pytest.fixture
async def two_repos_with_shared_files(
    session_factory: Callable[[], AsyncSession],
    sample_file: GitFile,
    unique_file: GitFile,
    another_unique_file: GitFile,
) -> tuple[
    GitRepo, GitRepo, GitCommit, GitCommit, GitCommit, GitFile, GitFile, GitFile
]:
    """Create two repositories with commits that share some files."""
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)

    # Create two repositories
    repo1 = GitRepo(
        id=None,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo1"),
        remote_uri=AnyUrl("https://github.com/test/repo1.git"),
        tracking_branch=None,
        num_commits=2,
        num_branches=0,
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
        num_branches=0,
        num_tags=0,
    )
    repo2 = await repo_repository.save(repo2)

    assert repo1.id is not None
    assert repo2.id is not None

    # Create commits with different file combinations:
    # - commit1 (repo1): has sample_file + unique_file
    # - commit2 (repo1): has sample_file + another_unique_file
    # - commit3 (repo2): has sample_file (SHARED with repo1 commits!)

    commit1 = GitCommit(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        commit_sha="commit1_sha_abc",
        date=datetime.now(UTC),
        message="First commit in repo1",
        parent_commit_sha=None,
        files=[sample_file, unique_file],  # Shared file + unique file
        author="Author1",
    )

    commit2 = GitCommit(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        commit_sha="commit2_sha_def",
        date=datetime.now(UTC),
        message="Second commit in repo1",
        parent_commit_sha=commit1.commit_sha,
        files=[sample_file, another_unique_file],  # Same shared + different unique
        author="Author1",
    )

    commit3 = GitCommit(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        commit_sha="commit3_sha_ghi",
        date=datetime.now(UTC),
        message="Commit in repo2",
        parent_commit_sha=None,
        files=[sample_file],  # Same shared file as repo1 commits!
        author="Author2",
    )

    # Save all commits
    await commit_repository.save_bulk([commit1, commit2], repo1.id)
    await commit_repository.save_bulk([commit3], repo2.id)

    return (
        repo1, repo2, commit1, commit2, commit3,
        sample_file, unique_file, another_unique_file
    )


class TestDeleteByRepoId:
    """Test delete_by_repo_id() method for DDD compliance and file sharing logic."""

    async def test_only_deletes_commits_and_files_it_controls(
        self,
        commit_repository: SqlAlchemyGitCommitRepository,
        two_repos_with_shared_files: tuple[
            GitRepo, GitRepo, GitCommit, GitCommit, GitCommit,
            GitFile, GitFile, GitFile
        ],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete_by_repo_id() only deletes commits it controls, not repos.

        This test verifies DDD compliance: the commit repository should only delete
        commits and their associated files, but NOT repositories (which are controlled
        by the git repository).
        """
        (repo1, repo2, commit1, commit2, commit3,
         sample_file, unique_file, another_unique_file) = two_repos_with_shared_files

        # Verify initial state exists
        async with session_factory() as session:
            # Check both repos exist
            repo_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos")
            )
            assert repo_count == 2

            # Check all commits exist
            commit_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            assert commit_count == 3

            # Check commit files exist:
            # commit1: 2 files, commit2: 2 files, commit3: 1 file = 5 total
            file_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_commit_files")
            )
            assert file_count == 5

        # Delete commits for repo1 only
        assert repo1.id is not None
        await commit_repository.delete_by_repo_id(repo1.id)

        # Verify only repo1's commits were deleted, not repos or repo2's commits
        async with session_factory() as session:
            # Both repos should still exist (commit repository doesn't control repos)
            repo_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos")
            )
            assert repo_count == 2, "Repositories should not be deleted"

            # Only repo1's commits should be deleted
            remaining_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            assert remaining_commits == 1, "Only repo2's commit should remain"

            # Verify it's repo2's commit that remains
            remaining_commit_sha = await session.scalar(
                text("SELECT commit_sha FROM git_commits")
            )
            assert remaining_commit_sha == commit3.commit_sha

    async def test_problematic_file_deletion_behavior(
        self,
        commit_repository: SqlAlchemyGitCommitRepository,
        two_repos_with_shared_files: tuple[
            GitRepo, GitRepo, GitCommit, GitCommit, GitCommit,
            GitFile, GitFile, GitFile
        ],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that exposes the CURRENT PROBLEMATIC behavior with file deletion.

        This test documents the current bug: when deleting commits from repo1,
        the current implementation incorrectly deletes ALL files associated with
        those commits, even if other commits (from other repos) still reference them.

        The shared file should NOT be deleted because commit3 still references it.
        """
        (repo1, repo2, commit1, commit2, commit3,
         sample_file, unique_file, another_unique_file) = two_repos_with_shared_files

        # Verify shared file exists in multiple commits before deletion
        async with session_factory() as session:
            shared_file_refs = await session.scalar(
                text("""
                SELECT COUNT(*) FROM git_commit_files
                WHERE path = :path AND blob_sha = :blob_sha
                """),
                {"path": sample_file.path, "blob_sha": sample_file.blob_sha}
            )
            assert shared_file_refs == 3, "Shared file should have 3 references"

        # Delete commits for repo1
        assert repo1.id is not None
        await commit_repository.delete_by_repo_id(repo1.id)

        # Check what happened to files
        async with session_factory() as session:
            # CURRENT PROBLEMATIC BEHAVIOR: All files from repo1 commits are deleted,
            # even though the shared file is still needed by commit3 from repo2
            remaining_files = await session.scalar(
                text("SELECT COUNT(*) FROM git_commit_files")
            )

            # This documents the current bug - it deletes too many files
            assert remaining_files <= 1, (
                "Current implementation incorrectly deletes shared files. "
                "This test documents the bug that needs to be fixed."
            )

            # Check if shared file still exists for commit3
            shared_file_refs = await session.scalar(
                text("""
                SELECT COUNT(*) FROM git_commit_files
                WHERE path = :path AND blob_sha = :blob_sha
                """),
                {"path": sample_file.path, "blob_sha": sample_file.blob_sha}
            )

            # Current implementation: shared_file_refs will be 0 (BUG!)
            # Correct implementation: shared_file_refs should be 1 (for commit3)
            if shared_file_refs == 0:
                pytest.fail(
                    "BUG DETECTED: Shared file was incorrectly deleted! "
                    "The shared file should still exist for commit3 from repo2, "
                    "but the implementation deleted it with repo1's commits."
                )

    async def test_correct_file_deletion_behavior_specification(
        self,
        commit_repository: SqlAlchemyGitCommitRepository,
        two_repos_with_shared_files: tuple[
            GitRepo, GitRepo, GitCommit, GitCommit, GitCommit,
            GitFile, GitFile, GitFile
        ],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that specifies the CORRECT file deletion behavior.

        This test defines what the CORRECT implementation should do:
        1. Delete only commits from the specified repository
        2. Delete files ONLY when no other commits reference them
        3. Keep files that are still referenced by commits in other repositories

        NOTE: This test will FAIL with the current implementation and should PASS
        once the file deletion logic is fixed.
        """
        (repo1, repo2, commit1, commit2, commit3,
         sample_file, unique_file, another_unique_file) = two_repos_with_shared_files

        # Delete commits for repo1
        assert repo1.id is not None
        await commit_repository.delete_by_repo_id(repo1.id)

        # Verify correct behavior
        async with session_factory() as session:
            # Only repo1's commits should be deleted
            remaining_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            assert remaining_commits == 1, "Only repo2's commit should remain"

            # CORRECT BEHAVIOR: Only 1 file should remain (shared file for commit3)
            remaining_files = await session.scalar(
                text("SELECT COUNT(*) FROM git_commit_files")
            )
            assert remaining_files == 1, (
                "Only the shared file for commit3 should remain. "
                "Unique files should be deleted since no other commits reference them."
            )

            # CORRECT BEHAVIOR: Shared file should still exist for commit3
            shared_file_refs = await session.scalar(
                text("""
                SELECT COUNT(*) FROM git_commit_files
                WHERE path = :path AND blob_sha = :blob_sha
                """),
                {"path": sample_file.path, "blob_sha": sample_file.blob_sha}
            )
            assert shared_file_refs == 1, (
                "Shared file should still exist for commit3 from repo2"
            )

            # CORRECT BEHAVIOR: Unique files should be deleted
            unique_file_refs = await session.scalar(
                text("""
                SELECT COUNT(*) FROM git_commit_files
                WHERE path = :path AND blob_sha = :blob_sha
                """),
                {"path": unique_file.path, "blob_sha": unique_file.blob_sha}
            )
            assert unique_file_refs == 0, (
                "Unique file should be deleted since no commits reference it"
            )

            another_unique_refs = await session.scalar(
                text("""
                SELECT COUNT(*) FROM git_commit_files
                WHERE path = :path AND blob_sha = :blob_sha
                """),
                {
                    "path": another_unique_file.path,
                    "blob_sha": another_unique_file.blob_sha
                }
            )
            assert another_unique_refs == 0, (
                "Another unique file should be deleted since no commits reference it"
            )

    async def test_deletes_only_commits_for_specified_repo(
        self,
        commit_repository: SqlAlchemyGitCommitRepository,
        two_repos_with_shared_files: tuple[
            GitRepo, GitRepo, GitCommit, GitCommit, GitCommit,
            GitFile, GitFile, GitFile
        ],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete_by_repo_id() only affects the specified repository."""
        (repo1, repo2, commit1, commit2, commit3,
         sample_file, unique_file, another_unique_file) = two_repos_with_shared_files

        # Delete commits for repo2 (which has only 1 commit)
        assert repo2.id is not None
        await commit_repository.delete_by_repo_id(repo2.id)

        # Verify only repo2's commits were affected
        async with session_factory() as session:
            # Repo1's commits should remain
            repo1_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits WHERE repo_id = :repo_id"),
                {"repo_id": repo1.id}
            )
            assert repo1_commits == 2, "Repo1's commits should remain untouched"

            # Repo2's commits should be gone
            repo2_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits WHERE repo_id = :repo_id"),
                {"repo_id": repo2.id}
            )
            assert repo2_commits == 0, "Repo2's commits should be deleted"

    async def test_handles_nonexistent_repo_gracefully(
        self,
        commit_repository: SqlAlchemyGitCommitRepository,
    ) -> None:
        """Test that delete_by_repo_id() handles non-existent repo IDs gracefully."""
        # Should not raise an exception when deleting commits for non-existent repo
        await commit_repository.delete_by_repo_id(99999)

    async def test_handles_repo_with_no_commits_gracefully(
        self,
        commit_repository: SqlAlchemyGitCommitRepository,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete_by_repo_id() handles repos with no commits gracefully."""
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

        # Should not raise an exception when deleting commits for empty repo
        await commit_repository.delete_by_repo_id(empty_repo.id)


class TestCountByRepoId:
    """Test count_by_repo_id() method."""

    async def test_returns_correct_count(
        self,
        commit_repository: SqlAlchemyGitCommitRepository,
        two_repos_with_shared_files: tuple[
            GitRepo, GitRepo, GitCommit, GitCommit, GitCommit,
            GitFile, GitFile, GitFile
        ],
    ) -> None:
        """Test that count_by_repo_id() returns the correct count."""
        (repo1, repo2, commit1, commit2, commit3,
         sample_file, unique_file, another_unique_file) = two_repos_with_shared_files

        assert repo1.id is not None
        assert repo2.id is not None

        # Repo1 should have 2 commits
        count1 = await commit_repository.count_by_repo_id(repo1.id)
        assert count1 == 2

        # Repo2 should have 1 commit
        count2 = await commit_repository.count_by_repo_id(repo2.id)
        assert count2 == 1

    async def test_returns_zero_for_nonexistent_repo(
        self,
        commit_repository: SqlAlchemyGitCommitRepository,
    ) -> None:
        """Test that count_by_repo_id() returns 0 for non-existent repo."""
        count = await commit_repository.count_by_repo_id(99999)
        assert count == 0


class TestDeleteByRepoIdWithSnippetFiles:
    """Test delete_by_repo_id() method with snippet file associations."""

    async def test_deletes_snippet_file_associations_before_commit_files(
        self,
        commit_repository: SqlAlchemyGitCommitRepository,
        two_repos_with_shared_files: tuple[
            GitRepo, GitRepo, GitCommit, GitCommit, GitCommit,
            GitFile, GitFile, GitFile
        ],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that snippet file associations are deleted before commit files.

        This test ensures the foreign key constraint error doesn't occur by creating
        SnippetV2File records that reference git_commit_files, then verifying that
        the deletion happens in the correct order.
        """
        (repo1, repo2, commit1, commit2, commit3,
         sample_file, unique_file, another_unique_file) = two_repos_with_shared_files

        # Create snippet file associations that reference the commit files
        from kodit.infrastructure.sqlalchemy import entities as db_entities

        async with session_factory() as session:
            # Create some snippet records first
            snippet1 = db_entities.SnippetV2(
                sha="test_snippet_1",
                content="test content 1",
                extension="py",
            )
            snippet2 = db_entities.SnippetV2(
                sha="test_snippet_2",
                content="test content 2",
                extension="py",
            )
            session.add(snippet1)
            session.add(snippet2)
            await session.commit()

            # Create snippet file associations that reference commit files
            # These create foreign key dependencies:
            # snippet_v2_files -> git_commit_files
            snippet_file_1 = db_entities.SnippetV2File(
                snippet_sha="test_snippet_1",
                blob_sha=sample_file.blob_sha,
                commit_sha=commit1.commit_sha,
                file_path=sample_file.path,
            )
            snippet_file_2 = db_entities.SnippetV2File(
                snippet_sha="test_snippet_2",
                blob_sha=unique_file.blob_sha,
                commit_sha=commit1.commit_sha,
                file_path=unique_file.path,
            )
            snippet_file_3 = db_entities.SnippetV2File(
                snippet_sha="test_snippet_1",
                blob_sha=sample_file.blob_sha,
                commit_sha=commit2.commit_sha,
                file_path=sample_file.path,
            )
            session.add(snippet_file_1)
            session.add(snippet_file_2)
            session.add(snippet_file_3)
            await session.commit()

        # Verify the associations were created
        async with session_factory() as session:
            snippet_file_count = await session.scalar(
                text("SELECT COUNT(*) FROM snippet_v2_files")
            )
            assert snippet_file_count == 3, "Should have 3 snippet file associations"

            commit_file_count = await session.scalar(
                text(
                    "SELECT COUNT(*) FROM git_commit_files "
                    "WHERE commit_sha IN (:commit1, :commit2)"
                ),
                {"commit1": commit1.commit_sha, "commit2": commit2.commit_sha}
            )
            assert commit_file_count > 0, "Should have commit files for repo1 commits"

        # Delete commits for repo1 - this should succeed without foreign key errors
        # because the implementation deletes snippet_v2_files before git_commit_files
        assert repo1.id is not None
        await commit_repository.delete_by_repo_id(repo1.id)

        # Verify the deletion worked correctly
        async with session_factory() as session:
            # Snippet file associations for repo1 commits should be deleted
            remaining_snippet_files = await session.scalar(
                text(
                    "SELECT COUNT(*) FROM snippet_v2_files "
                    "WHERE commit_sha IN (:commit1, :commit2)"
                ),
                {"commit1": commit1.commit_sha, "commit2": commit2.commit_sha}
            )
            assert remaining_snippet_files == 0, (
                "Snippet file associations should be deleted"
            )

            # Commit files for repo1 should be deleted
            remaining_commit_files = await session.scalar(
                text(
                    "SELECT COUNT(*) FROM git_commit_files "
                    "WHERE commit_sha IN (:commit1, :commit2)"
                ),
                {"commit1": commit1.commit_sha, "commit2": commit2.commit_sha}
            )
            assert remaining_commit_files == 0, "Commit files should be deleted"

            # Repo1 commits should be deleted
            remaining_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits WHERE repo_id = :repo_id"),
                {"repo_id": repo1.id}
            )
            assert remaining_commits == 0, "Repo1 commits should be deleted"

            # Snippets themselves should still exist (they're not owned by commits)
            remaining_snippets = await session.scalar(
                text(
                    "SELECT COUNT(*) FROM snippets_v2 "
                    "WHERE sha IN (:snippet1, :snippet2)"
                ),
                {"snippet1": "test_snippet_1", "snippet2": "test_snippet_2"}
            )
            assert remaining_snippets == 2, "Snippets should still exist"

            # Repo2's data should be untouched
            repo2_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits WHERE repo_id = :repo_id"),
                {"repo_id": repo2.id}
            )
            assert repo2_commits == 1, "Repo2's commits should remain"

