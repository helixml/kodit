"""Tests for repository listing API endpoint."""

import tempfile
from collections.abc import AsyncIterator, Callable
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from pathlib import Path

import git
import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.server_factory import ServerFactory
from kodit.application.services.repository_sync_service import RepositorySyncService
from kodit.config import AppContext
from kodit.domain.entities.git import GitCommit, TrackingConfig
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.git_repository_service import GitRepositoryScanner
from kodit.infrastructure.api.v1.routers.repositories import (
    router as repositories_router,
)
from kodit.infrastructure.api.v1.schemas.context import AppLifespanState
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.sqlalchemy.git_branch_repository import (
    create_git_branch_repository,
)
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.git_tag_repository import (
    create_git_tag_repository,
)
from kodit.infrastructure.sqlalchemy.query import FilterOperator, QueryBuilder


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
async def test_repository_listing_with_commit_count(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test repository listing returns correct commit count after indexing."""
    # Setup repositories
    git_repo_repository = create_git_repo_repository(session_factory)
    git_commit_repository = create_git_commit_repository(session_factory)
    git_branch_repository = create_git_branch_repository(session_factory)
    git_tag_repository = create_git_tag_repository(session_factory)

    sync_service = RepositorySyncService(
        scanner=GitRepositoryScanner(GitPythonAdapter()),
        git_commit_repository=git_commit_repository,
        git_branch_repository=git_branch_repository,
        git_tag_repository=git_tag_repository,
    )

    with tempfile.TemporaryDirectory() as tmpdir:
        # Create git repo with initial commit
        remote_path = Path(tmpdir) / "remote.git"
        git.Repo.init(remote_path, bare=True, initial_branch="main")
        repo_path = Path(tmpdir) / "repo"
        git_repo = git.Repo.clone_from(str(remote_path), str(repo_path))

        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        test_file = repo_path / "test.txt"
        test_file.write_text("initial content")
        git_repo.index.add(["test.txt"])
        commit1 = git_repo.index.commit("Initial commit")
        git_repo.git.push("origin", "HEAD:main")

        # Save repository to database
        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)
        assert repo.id is not None

        # Index first commit and sync
        await git_commit_repository.save(
            GitCommit(
                commit_sha=commit1.hexsha,
                repo_id=repo.id,
                message=str(commit1.message),
                author=str(commit1.author),
                date=datetime.fromtimestamp(commit1.committed_date, UTC),
            )
        )
        await sync_service.sync_branches_and_tags(repo)

        # Update counts and verify API returns num_commits=1
        repo.num_commits = await git_commit_repository.count(
            QueryBuilder().filter("repo_id", FilterOperator.EQ, repo.id)
        )
        repo.num_branches = await git_branch_repository.count(
            QueryBuilder().filter("repo_id", FilterOperator.EQ, repo.id)
        )
        await git_repo_repository.save(repo)

        response = await client.get("/api/v1/repositories")
        assert response.status_code == 200
        repo_data = response.json()["data"][0]
        assert repo_data["attributes"]["num_commits"] == 1

        # Add second commit
        test_file.write_text("updated content")
        git_repo.index.add(["test.txt"])
        commit2 = git_repo.index.commit("Second commit")
        git_repo.git.push("origin", "HEAD:main")

        # Index second commit and sync
        await git_commit_repository.save(
            GitCommit(
                commit_sha=commit2.hexsha,
                repo_id=repo.id,
                message=str(commit2.message),
                author=str(commit2.author),
                date=datetime.fromtimestamp(commit2.committed_date, UTC),
                parent_commit_sha=commit1.hexsha,
            )
        )
        await sync_service.sync_branches_and_tags(repo)

        # Update counts and verify API returns num_commits=2
        repo.num_commits = await git_commit_repository.count(
            QueryBuilder().filter("repo_id", FilterOperator.EQ, repo.id)
        )
        await git_repo_repository.save(repo)

        response = await client.get("/api/v1/repositories")
        assert response.status_code == 200
        repo_data = response.json()["data"][0]
        assert repo_data["attributes"]["num_commits"] == 2


@pytest.mark.asyncio
async def test_repository_listing_includes_timestamps(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test repository listing returns created_at and updated_at timestamps."""
    git_repo_repository = create_git_repo_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
        # Create git repo
        remote_path = Path(tmpdir) / "remote.git"
        git.Repo.init(remote_path, bare=True, initial_branch="main")
        repo_path = Path(tmpdir) / "repo"
        git_repo = git.Repo.clone_from(str(remote_path), str(repo_path))

        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        test_file = repo_path / "test.txt"
        test_file.write_text("initial content")
        git_repo.index.add(["test.txt"])
        git_repo.index.commit("Initial commit")
        git_repo.git.push("origin", "HEAD:main")

        # Save repository to database
        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/timestamps-repo.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)
        assert repo.id is not None

        # Verify timestamps are returned in API response
        response = await client.get("/api/v1/repositories")
        assert response.status_code == 200

        data = response.json()["data"]
        repo_data = next(
            r for r in data
            if "timestamps-repo" in r["attributes"]["remote_uri"]
        )

        # created_at and updated_at should not be null
        assert repo_data["attributes"]["created_at"] is not None, \
            "created_at should not be null"
        assert repo_data["attributes"]["updated_at"] is not None, \
            "updated_at should not be null"
