"""Tests for repository credential update scenario.

This test verifies the bug hypothesis:
When a repository is first created without credentials and later
re-created with credentials, the stored remote_uri should be updated
with the new credentials so that clone operations can authenticate.
"""

from collections.abc import AsyncIterator, Callable
from contextlib import asynccontextmanager

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.server_factory import ServerFactory
from kodit.config import AppContext
from kodit.infrastructure.api.middleware import auth
from kodit.infrastructure.api.v1 import dependencies
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
    ) as http_client:
        yield http_client


@pytest.mark.asyncio
async def test_creating_repo_with_credentials_updates_existing_repo(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test that re-creating a repository with credentials updates remote_uri.

    Scenario:
    1. Repository created without credentials (e.g., public repo)
    2. Repository becomes private / requires authentication
    3. User tries to create the same repo again with credentials
    4. The stored remote_uri should be updated with the new credentials
    """
    git_repo_repository = create_git_repo_repository(session_factory)

    # Step 1: Create repository without credentials
    response = await client.post(
        "/api/v1/repositories",
        json={
            "data": {
                "type": "repository",
                "attributes": {
                    "remote_uri": "https://github.com/test/private-repo.git",
                },
            }
        },
    )
    assert response.status_code == 201
    repo_id = int(response.json()["data"]["id"])

    # Verify the repo was created without credentials in remote_uri
    repo = await git_repo_repository.get(repo_id)
    assert repo is not None
    assert "token" not in str(repo.remote_uri)
    assert str(repo.remote_uri) == "https://github.com/test/private-repo.git"

    # Step 2: Try to create the same repo again WITH credentials
    # This simulates the case where the repo now requires authentication
    response = await client.post(
        "/api/v1/repositories",
        json={
            "data": {
                "type": "repository",
                "attributes": {
                    "remote_uri": "https://user:secret-token@github.com/test/private-repo.git",
                },
            }
        },
    )

    # Should return 200 (existing repo) not 201 (new repo)
    assert response.status_code == 200
    returned_repo_id = int(response.json()["data"]["id"])
    assert returned_repo_id == repo_id  # Same repo

    # Step 3: Verify the stored remote_uri was updated with credentials
    updated_repo = await git_repo_repository.get(repo_id)
    assert updated_repo is not None

    # Verify the remote_uri was updated with credentials
    assert "secret-token" in str(updated_repo.remote_uri), (
        f"Expected remote_uri to contain credentials, got: {updated_repo.remote_uri}"
    )


@pytest.mark.asyncio
async def test_sanitized_uri_remains_unchanged_when_credentials_added(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test that sanitized_remote_uri stays the same regardless of credentials.

    The sanitized URI is used as the business key for lookups and should
    remain consistent whether credentials are provided or not.
    """
    git_repo_repository = create_git_repo_repository(session_factory)

    # Create without credentials
    response = await client.post(
        "/api/v1/repositories",
        json={
            "data": {
                "type": "repository",
                "attributes": {
                    "remote_uri": "https://github.com/test/another-repo.git",
                },
            }
        },
    )
    assert response.status_code == 201
    repo_id = int(response.json()["data"]["id"])

    repo = await git_repo_repository.get(repo_id)
    assert repo is not None
    original_sanitized_uri = str(repo.sanitized_remote_uri)

    # Create again with credentials
    response = await client.post(
        "/api/v1/repositories",
        json={
            "data": {
                "type": "repository",
                "attributes": {
                    "remote_uri": "https://myuser:mypassword@github.com/test/another-repo.git",
                },
            }
        },
    )
    assert response.status_code == 200

    # Verify sanitized URI is unchanged
    updated_repo = await git_repo_repository.get(repo_id)
    assert updated_repo is not None
    assert str(updated_repo.sanitized_remote_uri) == original_sanitized_uri
