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

    # Override dependencies to use our test fixtures
    def override_app_context() -> AppContext:
        return app_context

    def override_server_factory() -> ServerFactory:
        return server_factory

    def override_auth() -> None:
        """No-op auth for testing."""
        return

    app = FastAPI(
        title="kodit API Test",
        lifespan=lambda app: api_test_lifespan(app, app_context, server_factory),
    )
    app.dependency_overrides[dependencies.get_app_context] = override_app_context
    app.dependency_overrides[dependencies.get_server_factory] = override_server_factory
    app.dependency_overrides[auth.api_key_auth] = override_auth
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
async def test_repository_listing_with_commit_count(  # noqa: PLR0915
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test that repository listing returns correct commit count after indexing.

    This test:
    1. Creates a git repo with one commit
    2. Indexes it and verifies num_commits=1 in API response
    3. Adds a second commit
    4. Re-indexes and verifies num_commits=2 in API response
    """
    # Create repositories
    git_repo_repository = create_git_repo_repository(session_factory)
    git_commit_repository = create_git_commit_repository(session_factory)
    git_branch_repository = create_git_branch_repository(session_factory)
    git_tag_repository = create_git_tag_repository(session_factory)

    # Create a real git repository with one commit
    with tempfile.TemporaryDirectory() as tmpdir:
        # Create bare remote repository
        remote_path = Path(tmpdir) / "remote.git"
        git.Repo.init(remote_path, bare=True, initial_branch="main")

        # Create working repository cloned from remote
        repo_path = Path(tmpdir) / "repo"
        git_repo = git.Repo.clone_from(str(remote_path), str(repo_path))
        with git_repo.config_writer() as cw:
            cw.set_value("user", "name", "Test User")
            cw.set_value("user", "email", "test@example.com")

        # Create first commit
        test_file = repo_path / "test.txt"
        test_file.write_text("initial content")
        git_repo.index.add(["test.txt"])
        commit1 = git_repo.index.commit("Initial commit")
        git_repo.git.push("origin", "HEAD:main")

        # Create domain entity and save to database
        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/repo.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)
        assert repo.id is not None

        # Manually index the first commit (simulating what the worker would do)
        initial_commit = GitCommit(
            commit_sha=commit1.hexsha,
            repo_id=repo.id,
            message=str(commit1.message),
            author=str(commit1.author),
            date=datetime.fromtimestamp(commit1.committed_date, UTC),
        )
        await git_commit_repository.save(initial_commit)

        # Sync branches for initial state
        sync_service = RepositorySyncService(
            scanner=GitRepositoryScanner(GitPythonAdapter()),
            git_commit_repository=git_commit_repository,
            git_branch_repository=git_branch_repository,
            git_tag_repository=git_tag_repository,
        )
        await sync_service.sync_branches_and_tags(repo)

        # Update repository counts
        repo.num_commits = await git_commit_repository.count(
            QueryBuilder().filter("repo_id", FilterOperator.EQ, repo.id)
        )
        repo.num_branches = await git_branch_repository.count(
            QueryBuilder().filter("repo_id", FilterOperator.EQ, repo.id)
        )
        repo = await git_repo_repository.save(repo)

        # Verify the repository has 1 commit indexed
        assert repo.num_commits == 1

        # Test 1: Verify API returns num_commits=1
        response = await client.get("/api/v1/repositories")
        assert response.status_code == 200
        data = response.json()
        assert "data" in data
        assert len(data["data"]) == 1

        repo_data = data["data"][0]
        assert repo_data["type"] == "repository"
        assert repo_data["id"] == str(repo.id)
        assert repo_data["attributes"]["num_commits"] == 1
        assert repo_data["attributes"]["tracking_branch"] == "main"

        # Create second commit
        test_file.write_text("updated content")
        git_repo.index.add(["test.txt"])
        commit2 = git_repo.index.commit("Second commit")
        git_repo.git.push("origin", "HEAD:main")

        # Manually index the second commit (simulating what the worker would do)
        assert repo.id is not None  # For type checking
        second_commit = GitCommit(
            commit_sha=commit2.hexsha,
            repo_id=repo.id,
            message=str(commit2.message),
            author=str(commit2.author),
            date=datetime.fromtimestamp(commit2.committed_date, UTC),
            parent_commit_sha=commit1.hexsha,
        )
        await git_commit_repository.save(second_commit)

        # Sync branches again to update head commit
        await sync_service.sync_branches_and_tags(repo)

        # Update repository counts
        repo.num_commits = await git_commit_repository.count(
            QueryBuilder().filter("repo_id", FilterOperator.EQ, repo.id)
        )
        repo = await git_repo_repository.save(repo)

        # Verify the repository now has 2 commits indexed
        assert repo.num_commits == 2

        # Test 2: Verify API returns num_commits=2
        response = await client.get("/api/v1/repositories")
        assert response.status_code == 200
        data = response.json()
        assert "data" in data
        assert len(data["data"]) == 1

        repo_data = data["data"][0]
        assert repo_data["type"] == "repository"
        assert repo_data["id"] == str(repo.id)
        assert repo_data["attributes"]["num_commits"] == 2
        assert repo_data["attributes"]["num_branches"] == 1
        assert repo_data["attributes"]["tracking_branch"] == "main"
