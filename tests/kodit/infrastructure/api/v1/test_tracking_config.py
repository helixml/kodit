"""Tests for repository tracking configuration API endpoints."""

import tempfile
from collections.abc import AsyncIterator, Callable
from contextlib import asynccontextmanager
from pathlib import Path

import git
import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.server_factory import ServerFactory
from kodit.config import AppContext
from kodit.domain.entities.git import TrackingConfig
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.infrastructure.api.v1.routers.repositories import (
    router as repositories_router,
)
from kodit.infrastructure.api.v1.schemas.context import AppLifespanState
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository


@asynccontextmanager
async def api_test_lifespan(
    _app: FastAPI,
    app_context: AppContext,
    server_factory: ServerFactory,
) -> AsyncIterator[AppLifespanState]:
    """Test lifespan without starting background services."""
    yield AppLifespanState(app_context=app_context, server_factory=server_factory)


@pytest.fixture
def test_app(
    app_context: AppContext,
    session_factory: Callable[[], AsyncSession],
) -> FastAPI:
    """Create a minimal FastAPI app for testing."""
    from kodit.infrastructure.api.middleware import auth
    from kodit.infrastructure.api.v1 import dependencies

    server_factory = ServerFactory(app_context, session_factory)

    app = FastAPI(
        title="kodit API Test",
        lifespan=lambda app: api_test_lifespan(app, app_context, server_factory),
    )
    app.dependency_overrides[dependencies.get_app_context] = lambda: app_context
    app.dependency_overrides[dependencies.get_server_factory] = lambda: server_factory
    app.dependency_overrides[auth.api_key_auth] = lambda: None
    app.include_router(repositories_router)
    return app


@pytest.fixture
async def client(test_app: FastAPI) -> AsyncIterator[AsyncClient]:
    """Create an async HTTP client for testing."""
    async with AsyncClient(
        transport=ASGITransport(app=test_app), base_url="http://test"
    ) as client:
        yield client


@pytest.mark.asyncio
async def test_get_tracking_config_branch_mode(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test GET tracking config returns branch mode correctly."""
    git_repo_repository = create_git_repo_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
        remote_path = Path(tmpdir) / "remote.git"
        git.Repo.init(remote_path, bare=True, initial_branch="main")
        repo_path = Path(tmpdir) / "repo"
        git.Repo.clone_from(str(remote_path), str(repo_path))

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/tracking-test.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="develop")
        repo = await git_repo_repository.save(repo)

        response = await client.get(f"/api/v1/repositories/{repo.id}/tracking-config")

        assert response.status_code == 200
        data = response.json()["data"]
        assert data["type"] == "tracking-config"
        assert data["attributes"]["mode"] == "branch"
        assert data["attributes"]["value"] == "develop"


@pytest.mark.asyncio
async def test_get_tracking_config_tag_mode(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test GET tracking config returns tag mode correctly."""
    git_repo_repository = create_git_repo_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
        remote_path = Path(tmpdir) / "remote.git"
        git.Repo.init(remote_path, bare=True, initial_branch="main")
        repo_path = Path(tmpdir) / "repo"
        git.Repo.clone_from(str(remote_path), str(repo_path))

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/tracking-tag-test.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="tag", name="")
        repo = await git_repo_repository.save(repo)

        response = await client.get(f"/api/v1/repositories/{repo.id}/tracking-config")

        assert response.status_code == 200
        data = response.json()["data"]
        assert data["type"] == "tracking-config"
        assert data["attributes"]["mode"] == "tag"
        assert data["attributes"]["value"] is None


@pytest.mark.asyncio
async def test_get_tracking_config_not_found(
    client: AsyncClient,
) -> None:
    """Test GET tracking config returns 404 for non-existent repository."""
    response = await client.get("/api/v1/repositories/99999/tracking-config")
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_put_tracking_config_branch_mode(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test PUT tracking config sets branch mode correctly."""
    git_repo_repository = create_git_repo_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
        remote_path = Path(tmpdir) / "remote.git"
        git.Repo.init(remote_path, bare=True, initial_branch="main")
        repo_path = Path(tmpdir) / "repo"
        git.Repo.clone_from(str(remote_path), str(repo_path))

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/tracking-update-test.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)

        response = await client.put(
            f"/api/v1/repositories/{repo.id}/tracking-config",
            json={
                "data": {
                    "type": "tracking-config",
                    "attributes": {"mode": "branch", "value": "feature-branch"},
                }
            },
        )

        assert response.status_code == 200
        data = response.json()["data"]
        assert data["attributes"]["mode"] == "branch"
        assert data["attributes"]["value"] == "feature-branch"

        updated_repo = await git_repo_repository.get(repo.id)
        assert updated_repo is not None
        assert updated_repo.tracking_config is not None
        assert updated_repo.tracking_config.type == "branch"
        assert updated_repo.tracking_config.name == "feature-branch"


@pytest.mark.asyncio
async def test_put_tracking_config_tag_mode(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test PUT tracking config sets tag mode correctly."""
    git_repo_repository = create_git_repo_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
        remote_path = Path(tmpdir) / "remote.git"
        git.Repo.init(remote_path, bare=True, initial_branch="main")
        repo_path = Path(tmpdir) / "repo"
        git.Repo.clone_from(str(remote_path), str(repo_path))

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/tracking-tag-update.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)

        response = await client.put(
            f"/api/v1/repositories/{repo.id}/tracking-config",
            json={
                "data": {
                    "type": "tracking-config",
                    "attributes": {"mode": "tag"},
                }
            },
        )

        assert response.status_code == 200
        data = response.json()["data"]
        assert data["attributes"]["mode"] == "tag"
        assert data["attributes"]["value"] is None

        updated_repo = await git_repo_repository.get(repo.id)
        assert updated_repo is not None
        assert updated_repo.tracking_config is not None
        assert updated_repo.tracking_config.type == "tag"


@pytest.mark.asyncio
async def test_put_tracking_config_not_found(
    client: AsyncClient,
) -> None:
    """Test PUT tracking config returns 404 for non-existent repository."""
    response = await client.put(
        "/api/v1/repositories/99999/tracking-config",
        json={
            "data": {
                "type": "tracking-config",
                "attributes": {"mode": "branch", "value": "main"},
            }
        },
    )
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_put_tracking_config_invalid_mode(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test PUT tracking config returns 422 for invalid mode."""
    git_repo_repository = create_git_repo_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
        remote_path = Path(tmpdir) / "remote.git"
        git.Repo.init(remote_path, bare=True, initial_branch="main")
        repo_path = Path(tmpdir) / "repo"
        git.Repo.clone_from(str(remote_path), str(repo_path))

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/tracking-invalid-test.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)

        response = await client.put(
            f"/api/v1/repositories/{repo.id}/tracking-config",
            json={
                "data": {
                    "type": "tracking-config",
                    "attributes": {"mode": "invalid_mode", "value": "main"},
                }
            },
        )

        assert response.status_code == 422
