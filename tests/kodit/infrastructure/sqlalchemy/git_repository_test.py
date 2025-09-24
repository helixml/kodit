"""Tests for SqlAlchemyGitRepoRepository."""

from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import pytest
from pydantic import AnyUrl
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitBranch, GitCommit, GitFile, GitRepo, GitTag
from kodit.infrastructure.sqlalchemy.git_repository import SqlAlchemyGitRepoRepository


@pytest.fixture
def repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemyGitRepoRepository:
    """Create a repository with a session factory."""
    return SqlAlchemyGitRepoRepository(session_factory)


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


@pytest.fixture
def sample_git_branch(sample_git_commit: GitCommit) -> GitBranch:
    """Create a sample git branch."""
    return GitBranch(
        repo_id=1,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        name="main",
        head_commit=sample_git_commit,
    )


@pytest.fixture
def sample_git_tag(sample_git_commit: GitCommit) -> GitTag:
    """Create a sample git tag."""
    return GitTag(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        name="v1.0.0",
        target_commit=sample_git_commit,
    )


@pytest.fixture
def sample_git_repo(
    sample_git_branch: GitBranch,
) -> GitRepo:
    """Create a sample git repository."""
    return GitRepo(
        id=None,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo"),
        tracking_branch=sample_git_branch,
        cloned_path=Path("/tmp/test_repo"),
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        last_scanned_at=datetime.now(UTC),
        num_tags=1,
        num_commits=1,  # One commit for testing
        num_branches=1,  # One branch for testing
    )


class TestSave:
    """Test save() method."""

    async def test_saves_new_git_repo_with_all_entities(
        self,
        repository: SqlAlchemyGitRepoRepository,
        sample_git_repo: GitRepo,
    ) -> None:
        """Test that save() creates a new git repo with all associated entities."""
        await repository.save(sample_git_repo)

        # Verify the repo was assigned an ID
        assert sample_git_repo.id is not None

        # Retrieve and verify
        result = await repository.get_by_id(sample_git_repo.id)
        assert result is not None
        assert str(result.sanitized_remote_uri) == str(
            sample_git_repo.sanitized_remote_uri
        )
        assert result.num_tags == 1
        assert result.num_branches == 1
        # Commits are no longer part of the GitRepo aggregate

    async def test_updates_existing_repo_by_uri(
        self,
        repository: SqlAlchemyGitRepoRepository,
        sample_git_repo: GitRepo,
    ) -> None:
        """Test that save() updates an existing repo found by URI."""
        # First save
        await repository.save(sample_git_repo)
        original_id = sample_git_repo.id

        # Create a new repo object with the same URI but different data
        # Need to create a minimal tracking branch for validation
        minimal_commit = GitCommit(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            commit_sha="minimal_commit_123",
            date=datetime.now(UTC),
            message="Minimal commit",
            files=[],
            author="Test",
        )
        minimal_branch = GitBranch(
            repo_id=None,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            name="minimal",
            head_commit=minimal_commit,
        )

        updated_repo = GitRepo(
            id=None,  # No ID to force lookup by URI
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            sanitized_remote_uri=sample_git_repo.sanitized_remote_uri,  # Same URI
            tracking_branch=minimal_branch,
            cloned_path=Path("/tmp/updated_repo"),  # Different path
            remote_uri=sample_git_repo.remote_uri,
            last_scanned_at=datetime.now(UTC),
            num_commits=2,  # Different commit count for testing
            num_branches=1,  # Different branch count for testing
            num_tags=0,  # No tags for testing
        )

        await repository.save(updated_repo)

        # Should have updated the existing repo
        assert updated_repo.id == original_id

        # Verify the update
        assert original_id is not None
        result = await repository.get_by_id(original_id)
        assert result is not None
        assert str(result.cloned_path) == "/tmp/updated_repo"

    async def test_updates_existing_repo_by_id(
        self,
        repository: SqlAlchemyGitRepoRepository,
        sample_git_repo: GitRepo,
    ) -> None:
        """Test that save() updates an existing repo found by ID."""
        # First save
        await repository.save(sample_git_repo)
        repo_id = sample_git_repo.id

        # Update the repo
        sample_git_repo.cloned_path = Path("/tmp/new_path")

        await repository.save(sample_git_repo)

        # Verify the update
        assert repo_id is not None
        result = await repository.get_by_id(repo_id)
        assert result is not None
        assert str(result.cloned_path) == "/tmp/new_path"


class TestGetById:
    """Test get_by_id() method."""

    async def test_raises_error_for_nonexistent_id(
        self,
        repository: SqlAlchemyGitRepoRepository,
    ) -> None:
        """Test that get_by_id() raises ValueError for non-existent ID."""
        with pytest.raises(ValueError, match="Repository with ID 99999 not found"):
            await repository.get_by_id(99999)

    async def test_returns_complete_repo_with_all_associations(
        self,
        repository: SqlAlchemyGitRepoRepository,
        sample_git_repo: GitRepo,
    ) -> None:
        """Test that get_by_id() returns complete repo with all associations."""
        await repository.save(sample_git_repo)

        assert sample_git_repo.id is not None
        result = await repository.get_by_id(sample_git_repo.id)
        assert result is not None
        assert result.id == sample_git_repo.id
        assert result.num_branches == 1
        assert result.num_tags == 1
        # Tracking branch loading depends on having branches saved via GitBranchRepo
        # In this isolated test, we only test the core repo persistence
        # Complex relationships are tested in integration tests


class TestGetByUri:
    """Test get_by_uri() method."""

    async def test_raises_error_for_nonexistent_uri(
        self,
        repository: SqlAlchemyGitRepoRepository,
    ) -> None:
        """Test that get_by_uri() raises ValueError for non-existent URI."""
        with pytest.raises(ValueError, match="Repository .* not found"):
            await repository.get_by_uri(AnyUrl("https://github.com/nonexistent/repo"))

    async def test_returns_repo_by_uri(
        self,
        repository: SqlAlchemyGitRepoRepository,
        sample_git_repo: GitRepo,
    ) -> None:
        """Test that get_by_uri() returns repo by sanitized URI."""
        await repository.save(sample_git_repo)

        result = await repository.get_by_uri(sample_git_repo.sanitized_remote_uri)
        assert result is not None
        assert str(result.sanitized_remote_uri) == str(
            sample_git_repo.sanitized_remote_uri
        )
        assert result.id == sample_git_repo.id


class TestDelete:
    """Test delete() method."""

    async def test_deletes_existing_repo(
        self,
        repository: SqlAlchemyGitRepoRepository,
    ) -> None:
        """Test that delete() removes an existing repo."""
        # Create a simple repo without complex relationships
        simple_repo = GitRepo(
            id=None,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            remote_uri=AnyUrl("https://github.com/simple/repo"),
            sanitized_remote_uri=AnyUrl("https://github.com/simple/repo"),
            tracking_branch=None,
            num_commits=0,  # Simple repo with no commits
            num_branches=0,  # Simple repo with no branches
            num_tags=0,  # Simple repo with no tags
        )

        await repository.save(simple_repo)
        repo_uri = simple_repo.sanitized_remote_uri

        # Verify it exists
        result = await repository.get_by_uri(repo_uri)
        assert result is not None

        # Delete it
        deleted = await repository.delete(repo_uri)
        assert deleted is True

        # Verify it's gone
        with pytest.raises(ValueError, match="Repository .* not found"):
            await repository.get_by_uri(repo_uri)

    async def test_returns_false_for_nonexistent_repo(
        self,
        repository: SqlAlchemyGitRepoRepository,
    ) -> None:
        """Test that delete() returns False for non-existent repo."""
        deleted = await repository.delete(AnyUrl("https://github.com/nonexistent/repo"))
        assert deleted is False

    async def test_fails_to_delete_repo_with_foreign_key_relationships(
        self,
        repository: SqlAlchemyGitRepoRepository,
        sample_git_repo: GitRepo,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete() fails when repo has foreign key relationships.

        This test demonstrates the current problem: when a repository has
        commits, branches, and tags in the database, deletion fails due
        to foreign key constraints because the git repo repository tries
        to delete everything instead of just the repo itself.
        """
        from sqlalchemy import text

        from kodit.infrastructure.sqlalchemy.git_branch_repository import (
            create_git_branch_repository,
        )
        from kodit.infrastructure.sqlalchemy.git_commit_repository import (
            create_git_commit_repository,
        )
        from kodit.infrastructure.sqlalchemy.git_tag_repository import (
            create_git_tag_repository,
        )

        # First save the repo
        await repository.save(sample_git_repo)
        assert sample_git_repo.id is not None

        # Now add commits, branches, and tags using their respective repositories
        commit_repo = create_git_commit_repository(session_factory)
        branch_repo = create_git_branch_repository(session_factory)
        tag_repo = create_git_tag_repository(session_factory)

        # Save commits, branches, and tags to create foreign key relationships
        assert sample_git_repo.tracking_branch is not None
        await commit_repo.save_bulk([sample_git_repo.tracking_branch.head_commit],
                                   sample_git_repo.id)
        await branch_repo.save_bulk([sample_git_repo.tracking_branch],
                                   sample_git_repo.id)

        # Create and save a tag
        tag = GitTag(
            created_at=datetime.now(UTC),
            repo_id=sample_git_repo.id,
            name="v1.0.0",
            target_commit=sample_git_repo.tracking_branch.head_commit,
        )
        await tag_repo.save_bulk([tag], sample_git_repo.id)

        # Also manually insert some commit files to create more FK relationships
        async with session_factory() as session:
            await session.execute(
                text("""
                INSERT INTO git_commit_files
                (commit_sha, path, blob_sha, mime_type, extension, size, created_at)
                VALUES (:commit_sha, :path, :blob_sha, :mime_type,
                        :extension, :size, :created_at)
                """),
                {
                    "commit_sha": (
                        sample_git_repo.tracking_branch.head_commit.commit_sha
                    ),
                    "path": "test.py",
                    "blob_sha": "test_blob_sha",
                    "mime_type": "text/x-python",
                    "extension": "py",
                    "size": 100,
                    "created_at": datetime.now(UTC)
                }
            )
            await session.commit()

        # Now deletion should fail due to foreign key constraints
        # The DDD-compliant implementation only deletes the repo itself
        # Foreign key constraints prevent deletion when related entities exist
        from sqlalchemy.exc import IntegrityError
        with pytest.raises(IntegrityError) as exc_info:
            await repository.delete(sample_git_repo.sanitized_remote_uri)

        # Verify it's a foreign key constraint error
        error_msg = str(exc_info.value).lower()
        assert any(keyword in error_msg for keyword in [
            "foreign key",
            "constraint",
            "integrity"
        ]), f"Expected foreign key constraint error, got: {exc_info.value}"

    async def test_successfully_deletes_repo_when_no_foreign_keys_exist(
        self,
        repository: SqlAlchemyGitRepoRepository,
        sample_git_repo: GitRepo,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete() succeeds when all foreign key relationships are removed.

        This test demonstrates the correct DDD approach: when all related entities
        have been properly deleted by their respective repositories, the git repo
        repository can successfully delete just the repo itself.
        """
        from kodit.infrastructure.sqlalchemy.git_branch_repository import (
            create_git_branch_repository,
        )
        from kodit.infrastructure.sqlalchemy.git_commit_repository import (
            create_git_commit_repository,
        )
        from kodit.infrastructure.sqlalchemy.git_tag_repository import (
            create_git_tag_repository,
        )

        # First save the repo and related entities
        await repository.save(sample_git_repo)
        assert sample_git_repo.id is not None

        commit_repo = create_git_commit_repository(session_factory)
        branch_repo = create_git_branch_repository(session_factory)
        tag_repo = create_git_tag_repository(session_factory)

        # Save commits, branches, and tags
        assert sample_git_repo.tracking_branch is not None
        await commit_repo.save_bulk([sample_git_repo.tracking_branch.head_commit],
                                   sample_git_repo.id)
        await branch_repo.save_bulk([sample_git_repo.tracking_branch],
                                   sample_git_repo.id)

        tag = GitTag(
            created_at=datetime.now(UTC),
            repo_id=sample_git_repo.id,
            name="v1.0.0",
            target_commit=sample_git_repo.tracking_branch.head_commit,
        )
        await tag_repo.save_bulk([tag], sample_git_repo.id)

        # Now delete them in the correct order (following DDD principles)
        # 1. Delete branches and tags first (they reference commits)
        await branch_repo.delete_by_repo_id(sample_git_repo.id)
        await tag_repo.delete_by_repo_id(sample_git_repo.id)

        # 2. Delete commits (they reference the repo)
        await commit_repo.delete_by_repo_id(sample_git_repo.id)

        # 3. Delete tracking branches (they also reference the repo)
        async with session_factory() as session:
            from sqlalchemy import text
            await session.execute(
                text("DELETE FROM git_tracking_branches WHERE repo_id = :repo_id"),
                {"repo_id": sample_git_repo.id}
            )
            await session.commit()

        # 4. Finally, delete the repo itself (no more foreign key constraints)
        deleted = await repository.delete(sample_git_repo.sanitized_remote_uri)
        assert deleted is True

        # Verify the repo is gone
        with pytest.raises(ValueError, match="Repository .* not found"):
            await repository.get_by_uri(sample_git_repo.sanitized_remote_uri)


class TestListAll:
    """Test list_all() method."""

    async def test_returns_empty_list_when_no_repos(
        self,
        repository: SqlAlchemyGitRepoRepository,
    ) -> None:
        """Test that get_all() returns empty list when no repos exist."""
        result = await repository.get_all()
        assert result == []

    async def test_returns_all_repos(
        self,
        repository: SqlAlchemyGitRepoRepository,
        sample_git_repo: GitRepo,
    ) -> None:
        """Test that get_all() returns all repositories."""
        await repository.save(sample_git_repo)

        # Create and save another repo with minimal valid structure
        another_commit = GitCommit(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            commit_sha="another_commit_456",
            date=datetime.now(UTC),
            message="Another commit",
            files=[],
            author="Test",
        )
        another_branch = GitBranch(
            repo_id=None,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            name="another_main",
            head_commit=another_commit,
        )

        another_repo = GitRepo(
            id=None,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            sanitized_remote_uri=AnyUrl("https://github.com/test/another-repo"),
            tracking_branch=another_branch,
            cloned_path=Path("/tmp/another_repo"),
            remote_uri=AnyUrl("https://github.com/test/another-repo.git"),
            last_scanned_at=datetime.now(UTC),
            num_commits=3,  # Another repo with different commit count
            num_branches=1,  # Another repo with one branch
            num_tags=0,  # No tags for testing
        )
        await repository.save(another_repo)

        result = await repository.get_all()
        assert len(result) == 2

        # Verify both repos are returned (order may vary)
        uris = {str(repo.sanitized_remote_uri) for repo in result}
        assert "https://github.com/test/repo" in uris
        assert "https://github.com/test/another-repo" in uris


class TestDeleteWithTrackingBranchConstraints:
    """Test deletion with git_tracking_branches foreign key constraints."""

    async def test_successfully_deletes_repo_with_tracking_branch_cleanup(
        self,
        repository: SqlAlchemyGitRepoRepository,
        sample_git_repo: GitRepo,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that deletion succeeds when git_tracking_branches are cleaned up first.

        This test verifies that the repository deletion process now properly
        handles the git_tracking_branches foreign key constraint by deleting
        tracking branches before deleting the repository.
        """
        # Save the repository first
        saved_repo = await repository.save(sample_git_repo)
        assert saved_repo.id is not None

        # The repository save process creates a git_tracking_branches record
        # Let's verify it exists
        async with session_factory() as session:
            tracking_branch_count = await session.scalar(
                text(
                    "SELECT COUNT(*) FROM git_tracking_branches "
                    "WHERE repo_id = :repo_id"
                ),
                {"repo_id": saved_repo.id}
            )
            assert tracking_branch_count == 1, "Should have one tracking branch record"

        # Now try to delete the repository - this should succeed now that we
        # clean up git_tracking_branches first
        deleted = await repository.delete(saved_repo.sanitized_remote_uri)
        assert deleted is True

        # Verify the repository was actually deleted
        async with session_factory() as session:
            remaining_repos = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos WHERE id = :repo_id"),
                {"repo_id": saved_repo.id}
            )
            assert remaining_repos == 0, "Repository should be deleted"

            # Verify tracking branches were also cleaned up
            remaining_tracking_branches = await session.scalar(
                text(
                    "SELECT COUNT(*) FROM git_tracking_branches "
                    "WHERE repo_id = :repo_id"
                ),
                {"repo_id": saved_repo.id}
            )
            assert remaining_tracking_branches == 0, (
                "Tracking branches should be deleted"
            )
