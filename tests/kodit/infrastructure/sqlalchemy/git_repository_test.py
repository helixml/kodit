"""Tests for SqlAlchemyGitRepoRepository."""

from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path

import pytest
from pydantic import AnyUrl
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
    sample_git_tag: GitTag,
) -> GitRepo:
    """Create a sample git repository."""
    return GitRepo(
        id=None,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo"),
        branches=[sample_git_branch],
        tags=[sample_git_tag],
        tracking_branch=sample_git_branch,
        cloned_path=Path("/tmp/test_repo"),
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        last_scanned_at=datetime.now(UTC),
        num_commits=1,  # One commit for testing
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
        assert len(result.branches) == 1
        assert len(result.tags) == 1
        assert result.branches[0].name == "main"
        assert result.tags[0].name == "v1.0.0"
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
            branches=[minimal_branch],  # Minimal branch
            tags=[],
            tracking_branch=minimal_branch,
            cloned_path=Path("/tmp/updated_repo"),  # Different path
            remote_uri=sample_git_repo.remote_uri,
            last_scanned_at=datetime.now(UTC),
            num_commits=2,  # Different commit count for testing
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
        assert len(result.branches) == 1
        assert len(result.tags) == 1
        assert result.tracking_branch is not None
        assert result.tracking_branch.name == "main"
        # Commits are no longer part of the GitRepo aggregate


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
            branches=[],
            tags=[],
            tracking_branch=None,
            num_commits=0,  # Simple repo with no commits
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
            branches=[another_branch],
            tags=[],
            tracking_branch=another_branch,
            cloned_path=Path("/tmp/another_repo"),
            remote_uri=AnyUrl("https://github.com/test/another-repo.git"),
            last_scanned_at=datetime.now(UTC),
            num_commits=3,  # Another repo with different commit count
        )
        await repository.save(another_repo)

        result = await repository.get_all()
        assert len(result) == 2

        # Verify both repos are returned (order may vary)
        uris = {str(repo.sanitized_remote_uri) for repo in result}
        assert "https://github.com/test/repo" in uris
        assert "https://github.com/test/another-repo" in uris
