"""Tests for SqlAlchemyGitTagRepository."""

from collections.abc import Callable
from datetime import UTC, datetime

import pytest
from pydantic import AnyUrl
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitFile, GitRepo, GitTag
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.git_tag_repository import (
    SqlAlchemyGitTagRepository,
    create_git_tag_repository,
)


@pytest.fixture
def tag_repository(
    session_factory: Callable[[], AsyncSession],
) -> SqlAlchemyGitTagRepository:
    """Create a tag repository with a session factory."""
    return SqlAlchemyGitTagRepository(session_factory)


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
def sample_git_tag(sample_git_commit: GitCommit) -> GitTag:
    """Create a sample git tag."""
    return GitTag(
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        name="v1.0.0",
        target_commit=sample_git_commit,
        repo_id=1,
    )


@pytest.fixture
async def sample_repo_with_data(
    session_factory: Callable[[], AsyncSession],
    sample_git_commit: GitCommit,
    sample_git_tag: GitTag,
) -> tuple[GitRepo, GitCommit, GitTag]:
    """Create a sample repository with commit and tag data."""
    # Create and save the repository
    repo_repository = create_git_repo_repository(session_factory)
    repo = GitRepo(
        id=None,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo"),
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        tracking_branch=None,
        num_commits=1,
        num_branches=0,
        num_tags=1,
    )
    repo = await repo_repository.save(repo)
    assert repo.id is not None

    # Save the commit
    commit_repository = create_git_commit_repository(session_factory)
    await commit_repository.save_bulk([sample_git_commit], repo.id)

    # Save the tag
    tag_repository = create_git_tag_repository(session_factory)
    await tag_repository.save_bulk([sample_git_tag], repo.id)

    return repo, sample_git_commit, sample_git_tag


class TestDeleteByRepoId:
    """Test delete_by_repo_id() method for DDD compliance."""

    async def test_only_deletes_tags_not_commits_or_repos(
        self,
        tag_repository: SqlAlchemyGitTagRepository,
        sample_repo_with_data: tuple[GitRepo, GitCommit, GitTag],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete_by_repo_id() only deletes tags, not commits or repos.

        This test verifies DDD compliance: the tag repository should only delete
        the entities it directly controls (GitTag), leaving commits and repos
        to be managed by their respective repositories.
        """
        repo, commit, tag = sample_repo_with_data

        # Verify initial data exists
        async with session_factory() as session:
            # Check repo exists
            repo_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos WHERE id = :repo_id"),
                {"repo_id": repo.id}
            )
            assert repo_count == 1

            # Check commit exists
            commit_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits WHERE commit_sha = :sha"),
                {"sha": commit.commit_sha}
            )
            assert commit_count == 1

            # Check tag exists
            tag_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_tags WHERE repo_id = :repo_id"),
                {"repo_id": repo.id}
            )
            assert tag_count == 1

        # Delete tags using the tag repository
        assert repo.id is not None
        await tag_repository.delete_by_repo_id(repo.id)

        # Verify only tags were deleted
        async with session_factory() as session:
            # Repo should still exist (tag repository doesn't control repos)
            repo_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos WHERE id = :repo_id"),
                {"repo_id": repo.id}
            )
            assert repo_count == 1, "Repository should not be deleted by tag repository"

            # Commit should still exist (tag repository doesn't control commits)
            commit_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits WHERE commit_sha = :sha"),
                {"sha": commit.commit_sha}
            )
            assert commit_count == 1, "Commit should not be deleted by tag repository"

            # Tags should be deleted (tag repository controls tags)
            tag_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_tags WHERE repo_id = :repo_id"),
                {"repo_id": repo.id}
            )
            assert tag_count == 0, "Tags should be deleted by tag repository"

    async def test_deletes_only_tags_for_specified_repo(
        self,
        tag_repository: SqlAlchemyGitTagRepository,
        session_factory: Callable[[], AsyncSession],
        sample_git_commit: GitCommit,
    ) -> None:
        """Test that delete_by_repo_id() only deletes tags for the specified repo."""
        # Create two repositories
        repo_repository = create_git_repo_repository(session_factory)

        repo1 = GitRepo(
            id=None,
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            sanitized_remote_uri=AnyUrl("https://github.com/test/repo1"),
            remote_uri=AnyUrl("https://github.com/test/repo1.git"),
            tracking_branch=None,
            num_commits=1,
            num_branches=0,
            num_tags=1,
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
            num_tags=1,
        )
        repo2 = await repo_repository.save(repo2)

        assert repo1.id is not None
        assert repo2.id is not None

        # Save commits to both repos
        commit_repository = create_git_commit_repository(session_factory)
        await commit_repository.save_bulk([sample_git_commit], repo1.id)

        commit2 = GitCommit(
            created_at=datetime.now(UTC),
            updated_at=datetime.now(UTC),
            commit_sha="commit_sha_789",
            date=datetime.now(UTC),
            message="Second commit",
            parent_commit_sha=None,
            files=[],
            author="Test Author",
        )
        await commit_repository.save_bulk([commit2], repo2.id)

        # Create tags for both repos
        tag1 = GitTag(
            created_at=datetime.now(UTC),
            name="v1.0.0",
            target_commit=sample_git_commit,
            repo_id=repo1.id,
        )
        tag2 = GitTag(
            created_at=datetime.now(UTC),
            name="v2.0.0",
            target_commit=commit2,
            repo_id=repo2.id,
        )

        assert repo1.id is not None
        assert repo2.id is not None
        await tag_repository.save_bulk([tag1], repo1.id)
        await tag_repository.save_bulk([tag2], repo2.id)

        # Verify both tags exist
        async with session_factory() as session:
            tag1_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_tags WHERE repo_id = :repo_id"),
                {"repo_id": repo1.id}
            )
            tag2_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_tags WHERE repo_id = :repo_id"),
                {"repo_id": repo2.id}
            )
            assert tag1_count == 1
            assert tag2_count == 1

        # Delete tags for repo1 only
        await tag_repository.delete_by_repo_id(repo1.id)

        # Verify only repo1's tags were deleted
        async with session_factory() as session:
            tag1_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_tags WHERE repo_id = :repo_id"),
                {"repo_id": repo1.id}
            )
            tag2_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_tags WHERE repo_id = :repo_id"),
                {"repo_id": repo2.id}
            )
            assert tag1_count == 0, "Repo1's tags should be deleted"
            assert tag2_count == 1, "Repo2's tags should remain"

    async def test_handles_nonexistent_repo_gracefully(
        self,
        tag_repository: SqlAlchemyGitTagRepository,
    ) -> None:
        """Test that delete_by_repo_id() handles non-existent repo IDs gracefully."""
        # Should not raise an exception when deleting tags for non-existent repo
        await tag_repository.delete_by_repo_id(99999)

    async def test_deletes_multiple_tags_for_repo(
        self,
        tag_repository: SqlAlchemyGitTagRepository,
        sample_repo_with_data: tuple[GitRepo, GitCommit, GitTag],
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that delete_by_repo_id() deletes all tags for a repository."""
        repo, commit, _tag = sample_repo_with_data

        # Add more tags to the same repo
        assert repo.id is not None
        additional_tags = [
            GitTag(
                created_at=datetime.now(UTC),
                name="v2.0.0",
                target_commit=commit,
                repo_id=repo.id,
            ),
            GitTag(
                created_at=datetime.now(UTC),
                name="v3.0.0",
                target_commit=commit,
                repo_id=repo.id,
            ),
        ]
        await tag_repository.save_bulk(additional_tags, repo.id)

        # Verify we have 3 tags total (1 from fixture + 2 additional)
        async with session_factory() as session:
            tag_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_tags WHERE repo_id = :repo_id"),
                {"repo_id": repo.id}
            )
            assert tag_count == 3

        # Delete all tags for the repo
        await tag_repository.delete_by_repo_id(repo.id)

        # Verify all tags were deleted
        async with session_factory() as session:
            tag_count = await session.scalar(
                text("SELECT COUNT(*) FROM git_tags WHERE repo_id = :repo_id"),
                {"repo_id": repo.id}
            )
            assert tag_count == 0


class TestCountByRepoId:
    """Test count_by_repo_id() method."""

    async def test_returns_correct_count(
        self,
        tag_repository: SqlAlchemyGitTagRepository,
        sample_repo_with_data: tuple[GitRepo, GitCommit, GitTag],
    ) -> None:
        """Test that count_by_repo_id() returns the correct count."""
        repo, commit, _tag = sample_repo_with_data

        # Should have 1 tag from the fixture
        assert repo.id is not None
        count = await tag_repository.count_by_repo_id(repo.id)
        assert count == 1

        # Add more tags
        additional_tags = [
            GitTag(
                created_at=datetime.now(UTC),
                name="v2.0.0",
                target_commit=commit,
                repo_id=repo.id,
            ),
            GitTag(
                created_at=datetime.now(UTC),
                name="v3.0.0",
                target_commit=commit,
                repo_id=repo.id,
            ),
        ]
        await tag_repository.save_bulk(additional_tags, repo.id)

        # Should now have 3 tags
        count = await tag_repository.count_by_repo_id(repo.id)
        assert count == 3

    async def test_returns_zero_for_nonexistent_repo(
        self,
        tag_repository: SqlAlchemyGitTagRepository,
    ) -> None:
        """Test that count_by_repo_id() returns 0 for non-existent repo."""
        count = await tag_repository.count_by_repo_id(99999)
        assert count == 0


class TestExists:
    """Test exists() method."""

    async def test_returns_true_for_existing_tag(
        self,
        tag_repository: SqlAlchemyGitTagRepository,
        sample_repo_with_data: tuple[GitRepo, GitCommit, GitTag],
    ) -> None:
        """Test that exists() returns True for existing tag."""
        repo, _commit, tag = sample_repo_with_data

        assert repo.id is not None
        exists = await tag_repository.exists(tag.name, repo.id)
        assert exists is True

    async def test_returns_false_for_nonexistent_tag(
        self,
        tag_repository: SqlAlchemyGitTagRepository,
        sample_repo_with_data: tuple[GitRepo, GitCommit, GitTag],
    ) -> None:
        """Test that exists() returns False for non-existent tag."""
        repo, _commit, _tag = sample_repo_with_data

        assert repo.id is not None
        exists = await tag_repository.exists("nonexistent-tag", repo.id)
        assert exists is False

    async def test_returns_false_for_nonexistent_repo(
        self,
        tag_repository: SqlAlchemyGitTagRepository,
    ) -> None:
        """Test that exists() returns False for non-existent repo."""
        exists = await tag_repository.exists("v1.0.0", 99999)
        assert exists is False
