"""Tests for SqlAlchemyGitRepoRepository."""

from collections.abc import Callable

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitRepo
from kodit.infrastructure.sqlalchemy.git_repository import SqlAlchemyGitRepoRepository
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


async def test_save_and_get_repository(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
) -> None:
    """Test saving and retrieving a repository."""
    repository = SqlAlchemyGitRepoRepository(session_factory)

    # Save repository
    saved_repo = await repository.save(sample_git_repo)
    assert saved_repo.id is not None
    assert saved_repo.sanitized_remote_uri == sample_git_repo.sanitized_remote_uri

    # Retrieve repository
    retrieved_repo = await repository.get(saved_repo.id)
    assert retrieved_repo is not None
    assert retrieved_repo.id == saved_repo.id
    assert retrieved_repo.sanitized_remote_uri == sample_git_repo.sanitized_remote_uri


async def test_get_by_uri(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
) -> None:
    """Test retrieving a repository by URI."""
    repository = SqlAlchemyGitRepoRepository(session_factory)

    # Save repository
    saved_repo = await repository.save(sample_git_repo)
    assert saved_repo.id is not None

    # Retrieve by URI
    retrieved_repo = await repository.find(
        QueryBuilder().filter(
            "sanitized_remote_uri",
            FilterOperator.EQ,
            sample_git_repo.sanitized_remote_uri,
        )
    )
    assert len(retrieved_repo) == 1
    assert retrieved_repo[0].id == saved_repo.id


async def test_get_all_repositories(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
) -> None:
    """Test retrieving all repositories."""
    repository = SqlAlchemyGitRepoRepository(session_factory)

    # Save multiple repositories
    repo1 = sample_git_repo
    repo2 = GitRepo(
        id=None,
        created_at=sample_git_repo.created_at,
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo2"),
        remote_uri=AnyUrl("https://github.com/test/repo2.git"),
        num_commits=1,
        num_branches=1,
        num_tags=1,
    )

    await repository.save(repo1)
    await repository.save(repo2)

    # Retrieve all repositories
    all_repos = await repository.find(QueryBuilder())
    assert len(all_repos) >= 2  # May have other repos from other tests

    # Check that our repos are in the list
    repo_uris = {repo.sanitized_remote_uri for repo in all_repos}
    assert repo1.sanitized_remote_uri in repo_uris
    assert repo2.sanitized_remote_uri in repo_uris


async def test_delete_repository(
    session_factory: Callable[[], AsyncSession],
    sample_git_repo: GitRepo,
) -> None:
    """Test deleting a repository."""
    repository = SqlAlchemyGitRepoRepository(session_factory)

    # Save repository
    saved_repo = await repository.save(sample_git_repo)
    assert saved_repo.id is not None

    # Delete repository
    await repository.delete(saved_repo)

    # Verify repository is gone
    with pytest.raises(ValueError, match="not found"):
        await repository.get(saved_repo.id)
