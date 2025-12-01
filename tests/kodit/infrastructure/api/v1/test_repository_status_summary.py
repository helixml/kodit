"""Tests for repository status summary API endpoint."""

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
from kodit.domain.entities import TaskStatus
from kodit.domain.entities.git import TrackingConfig
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.value_objects import ReportingState, TaskOperation, TrackableType
from kodit.infrastructure.api.v1.routers.repositories import (
    router as repositories_router,
)
from kodit.infrastructure.api.v1.schemas.context import AppLifespanState
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.task_status_repository import (
    create_task_status_repository,
)


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
async def test_status_summary_returns_pending_when_no_tasks(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test status summary returns pending when no tasks exist."""
    git_repo_repository = create_git_repo_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
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

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/status-repo.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)
        assert repo.id is not None

        response = await client.get(f"/api/v1/repositories/{repo.id}/status/summary")
        assert response.status_code == 200

        data = response.json()["data"]
        assert data["type"] == "repository_status_summary"
        assert data["id"] == str(repo.id)
        assert data["attributes"]["status"] == "pending"
        assert data["attributes"]["message"] == ""


@pytest.mark.asyncio
async def test_status_summary_returns_in_progress_when_task_running(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test status summary returns in_progress when a task is running."""
    git_repo_repository = create_git_repo_repository(session_factory)
    task_status_repository = create_task_status_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
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

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/progress-repo.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)
        assert repo.id is not None

        task = TaskStatus.create(
            operation=TaskOperation.CLONE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repo.id,
        )
        task.state = ReportingState.IN_PROGRESS
        await task_status_repository.save(task)

        response = await client.get(f"/api/v1/repositories/{repo.id}/status/summary")
        assert response.status_code == 200

        data = response.json()["data"]
        assert data["attributes"]["status"] == "in_progress"


@pytest.mark.asyncio
async def test_status_summary_returns_completed_when_all_tasks_done(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test status summary returns completed when all tasks are done."""
    git_repo_repository = create_git_repo_repository(session_factory)
    task_status_repository = create_task_status_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
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

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/completed-repo.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)
        assert repo.id is not None

        task = TaskStatus.create(
            operation=TaskOperation.CLONE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repo.id,
        )
        task.state = ReportingState.COMPLETED
        await task_status_repository.save(task)

        response = await client.get(f"/api/v1/repositories/{repo.id}/status/summary")
        assert response.status_code == 200

        data = response.json()["data"]
        assert data["attributes"]["status"] == "completed"


@pytest.mark.asyncio
async def test_status_summary_returns_failed_with_error_message(
    session_factory: Callable[[], AsyncSession],
    client: AsyncClient,
) -> None:
    """Test status summary returns failed with error message when a task fails."""
    git_repo_repository = create_git_repo_repository(session_factory)
    task_status_repository = create_task_status_repository(session_factory)

    with tempfile.TemporaryDirectory() as tmpdir:
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

        repo = GitRepoFactory.create_from_remote_uri(
            AnyUrl("https://github.com/test/failed-repo.git")
        )
        repo.cloned_path = repo_path
        repo.tracking_config = TrackingConfig(type="branch", name="main")
        repo = await git_repo_repository.save(repo)
        assert repo.id is not None

        task = TaskStatus.create(
            operation=TaskOperation.CLONE_REPOSITORY,
            trackable_type=TrackableType.KODIT_REPOSITORY,
            trackable_id=repo.id,
        )
        task.fail("Clone failed: network error")
        await task_status_repository.save(task)

        response = await client.get(f"/api/v1/repositories/{repo.id}/status/summary")
        assert response.status_code == 200

        data = response.json()["data"]
        assert data["attributes"]["status"] == "failed"
        assert data["attributes"]["message"] == "Clone failed: network error"


@pytest.mark.asyncio
async def test_status_summary_returns_404_for_unknown_repo(
    client: AsyncClient,
) -> None:
    """Test status summary returns 404 for unknown repository."""
    response = await client.get("/api/v1/repositories/99999/status/summary")
    assert response.status_code == 404
