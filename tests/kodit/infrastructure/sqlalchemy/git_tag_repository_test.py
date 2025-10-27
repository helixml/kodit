"""Tests for SqlAlchemyGitTagRepository."""

from collections.abc import Callable
from datetime import UTC, datetime

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitCommit, GitRepo, GitTag
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.git_tag_repository import (
    create_git_tag_repository,
)


@pytest.fixture
async def repo_with_tags(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
    sample_git_commit: GitCommit,
) -> tuple[GitRepo, list[GitTag]]:
    """Create a repository with tags for testing."""
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)
    tag_repository = create_git_tag_repository(session_factory)

    # Save repository and commit
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None
    await commit_repository.save_bulk([sample_git_commit])

    # Create tags
    tags = [
        GitTag(
            created_at=datetime.now(UTC),
            repo_id=saved_repo.id,
            name="v1.0.0",
            target_commit_sha=sample_git_commit.commit_sha,
        ),
        GitTag(
            created_at=datetime.now(UTC),
            repo_id=saved_repo.id,
            name="v1.1.0",
            target_commit_sha=sample_git_commit.commit_sha,
        ),
    ]

    await tag_repository.save_bulk(tags)
    return saved_repo, tags


class TestTagDeletion:
    """Test tag deletion functionality."""

    async def test_deletes_tags_only(
        self,
        session_factory: Callable[[], AsyncSession],
        repo_with_tags: tuple[GitRepo, list[GitTag]],
    ) -> None:
        """Test that delete_by_repo_id only deletes tags, not other entities."""
        tag_repository = create_git_tag_repository(session_factory)
        repo, tags = repo_with_tags

        # Verify initial state
        async with session_factory() as session:
            initial_tags = await session.scalar(text("SELECT COUNT(*) FROM git_tags"))
            initial_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            initial_repos = await session.scalar(text("SELECT COUNT(*) FROM git_repos"))

            assert initial_tags == 2
            assert initial_commits == 1
            assert initial_repos == 1

        # Delete tags
        assert repo.id is not None
        await tag_repository.delete_by_repo_id(repo.id)

        # Verify only tags were deleted
        async with session_factory() as session:
            remaining_tags = await session.scalar(text("SELECT COUNT(*) FROM git_tags"))
            remaining_commits = await session.scalar(
                text("SELECT COUNT(*) FROM git_commits")
            )
            remaining_repos = await session.scalar(
                text("SELECT COUNT(*) FROM git_repos")
            )

            assert remaining_tags == 0
            assert remaining_commits == 1  # Commits should remain
            assert remaining_repos == 1  # Repos should remain

    async def test_handles_nonexistent_repo(
        self,
        session_factory: Callable[[], AsyncSession],
    ) -> None:
        """Test that deleting tags for non-existent repo handles gracefully."""
        tag_repository = create_git_tag_repository(session_factory)

        # Should not raise an exception
        await tag_repository.delete_by_repo_id(99999)


async def test_save_and_get_tags(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
    sample_git_commit: GitCommit,
) -> None:
    """Test saving and retrieving tags."""
    tag_repository = create_git_tag_repository(session_factory)
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)

    # Save repository and commit
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None
    await commit_repository.save_bulk([sample_git_commit])

    # Create and save tag
    tag = GitTag(
        created_at=datetime.now(UTC),
        repo_id=saved_repo.id,
        name="v1.0.0",
        target_commit_sha=sample_git_commit.commit_sha,
    )
    await tag_repository.save(tag)

    # Retrieve tags
    retrieved_tags = await tag_repository.get_by_repo_id(saved_repo.id)
    assert len(retrieved_tags) == 1
    assert retrieved_tags[0].name == "v1.0.0"
    assert retrieved_tags[0].repo_id == saved_repo.id


async def test_save_multiple_tags(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
    sample_git_commit: GitCommit,
) -> None:
    """Test saving multiple tags for a repository."""
    tag_repository = create_git_tag_repository(session_factory)
    repo_repository = create_git_repo_repository(session_factory)
    commit_repository = create_git_commit_repository(session_factory)

    # Save repository and commit
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None
    await commit_repository.save_bulk([sample_git_commit])

    # Create multiple tags
    tags = [
        GitTag(
            created_at=datetime.now(UTC),
            repo_id=saved_repo.id,
            name="v1.0.0",
            target_commit_sha=sample_git_commit.commit_sha,
        ),
        GitTag(
            created_at=datetime.now(UTC),
            repo_id=saved_repo.id,
            name="v2.0.0",
            target_commit_sha=sample_git_commit.commit_sha,
        ),
    ]

    # Save all tags
    await tag_repository.save_bulk(tags)

    # Retrieve and verify
    retrieved_tags = await tag_repository.get_by_repo_id(saved_repo.id)
    assert len(retrieved_tags) == 2
    tag_names = {tag.name for tag in retrieved_tags}
    assert tag_names == {"v1.0.0", "v2.0.0"}


async def test_empty_repository_returns_empty_list(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
) -> None:
    """Test that querying tags for a repository with no tags returns empty list."""
    tag_repository = create_git_tag_repository(session_factory)
    repo_repository = create_git_repo_repository(session_factory)

    # Save repository without tags
    saved_repo = await repo_repository.save(sample_git_repo)
    assert saved_repo.id is not None

    # Query tags for the empty repository
    tags = await tag_repository.get_by_repo_id(saved_repo.id)
    assert tags == []


async def test_nonexistent_repository_returns_empty_list(
    session_factory: Callable[[], AsyncSession],
) -> None:
    """Test that querying tags for a non-existent repository returns empty list."""
    tag_repository = create_git_tag_repository(session_factory)

    # Query tags for a repository that doesn't exist
    tags = await tag_repository.get_by_repo_id(99999)
    assert tags == []
