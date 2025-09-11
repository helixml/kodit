"""Test cases for the repositories API router."""

from datetime import UTC
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from fastapi import HTTPException
from pydantic import AnyUrl

from kodit.domain.entities import GitBranch, GitCommit, GitRepo


@pytest.fixture
def mock_git_service():
    """Create a mock git application service."""
    service = MagicMock()
    service.repo_repository = AsyncMock()
    service.commit_repository = AsyncMock()
    service.branch_repository = AsyncMock()
    service.clone_and_map_repository = AsyncMock()
    service.update_repository = AsyncMock()
    service.rescan_existing_repository = AsyncMock()
    return service


@pytest.fixture
def sample_repo():
    """Create a sample GitRepo for testing."""
    from datetime import datetime
    from pathlib import Path

    commit = GitCommit(
        commit_sha="abc123",
        date=datetime.now(UTC),
        message="Test commit",
        parent_commit_sha="parent123",
        files=[]
    )

    branch = GitBranch(
        name="main",
        head_commit=commit
    )

    return GitRepo(
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo"),
        branches=[branch],
        tracking_branch=branch,
        cloned_path=Path("/tmp/repo"),
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        last_scanned_at=datetime.now(UTC)
    )


async def test_list_repositories_empty(mock_git_service) -> None:
    """Test listing repositories when none exist."""
    mock_git_service.repo_repository.get_all.return_value = []

    with patch("kodit.infrastructure.api.v1.routers.repositories.GitAppServiceDep",
               return_value=mock_git_service):
        from kodit.infrastructure.api.v1.routers.repositories import list_repositories
        result = await list_repositories(mock_git_service)

    assert result.data == []
    mock_git_service.repo_repository.get_all.assert_called_once()


async def test_list_repositories_with_data(mock_git_service, sample_repo) -> None:
    """Test listing repositories with existing repos."""
    mock_git_service.repo_repository.get_all.return_value = [sample_repo]

    with patch("kodit.infrastructure.api.v1.routers.repositories.GitAppServiceDep",
               return_value=mock_git_service):
        from kodit.infrastructure.api.v1.routers.repositories import list_repositories
        result = await list_repositories(mock_git_service)

    assert len(result.data) == 1
    assert result.data[0].id == str(sample_repo.sanitized_remote_uri)
    assert result.data[0].attributes.default_branch == "main"


async def test_clone_repository_success(mock_git_service, sample_repo) -> None:
    """Test successful repository cloning."""
    mock_git_service.clone_and_map_repository.return_value = sample_repo

    from kodit.infrastructure.api.v1.schemas.repository import (
        RepositoryCreateAttributes,
        RepositoryCreateData,
        RepositoryCreateRequest,
    )

    request = RepositoryCreateRequest(
        data=RepositoryCreateData(
            type="repository",
            attributes=RepositoryCreateAttributes(
                remote_uri=AnyUrl("https://github.com/test/new-repo.git")
            )
        )
    )

    with patch("kodit.infrastructure.api.v1.routers.repositories.GitAppServiceDep",
               return_value=mock_git_service):
        from kodit.infrastructure.api.v1.routers.repositories import clone_repository
        result = await clone_repository(request, mock_git_service)

    assert result.data.id == str(sample_repo.sanitized_remote_uri)
    mock_git_service.clone_and_map_repository.assert_called_once()


async def test_get_repository_not_found(mock_git_service) -> None:
    """Test getting a non-existent repository."""
    mock_git_service.repo_repository.get_by_uri.return_value = None

    with pytest.raises(HTTPException) as exc_info:
        from kodit.infrastructure.api.v1.routers.repositories import get_repository
        await get_repository("https://github.com/test/nonexistent", mock_git_service)

    assert exc_info.value.status_code == 404
    assert "Repository not found" in str(exc_info.value.detail)


async def test_delete_repository_success(mock_git_service, sample_repo) -> None:
    """Test successful repository deletion."""
    mock_git_service.repo_repository.get_by_uri.return_value = sample_repo
    mock_git_service.repo_repository.delete.return_value = True

    with patch("kodit.infrastructure.api.v1.routers.repositories.GitAppServiceDep",
               return_value=mock_git_service):
        from kodit.infrastructure.api.v1.routers.repositories import delete_repository
        await delete_repository("https://github.com/test/repo", mock_git_service)

    mock_git_service.repo_repository.delete.assert_called_once()
