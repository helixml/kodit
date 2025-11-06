"""Performance regression tests for clone and scan operations.

This test measures the performance of cloning and scanning a real repository
(helixml/helix) to track performance over time and identify regressions.
"""

import tempfile
import time
from collections.abc import AsyncGenerator, Callable
from pathlib import Path

import pytest
from pydantic import AnyUrl
from sqlalchemy import text
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from kodit.application.factories.reporting_factory import create_noop_operation
from kodit.config import AppContext
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.services.git_repository_service import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.sqlalchemy.entities import Base
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository


@pytest.fixture
async def performance_engine() -> AsyncGenerator[AsyncEngine, None]:
    """Create a test database engine for performance tests."""
    with tempfile.TemporaryDirectory() as tmp_dir:
        db_path = Path(tmp_dir) / "performance_test.db"
        engine = create_async_engine(
            f"sqlite+aiosqlite:///{db_path}",
            echo=False,
            future=True,
        )

        async with engine.begin() as conn:
            await conn.execute(text("PRAGMA foreign_keys = ON"))
            await conn.run_sync(Base.metadata.create_all)

        yield engine

        async with engine.begin() as conn:
            await conn.run_sync(Base.metadata.drop_all)

        await engine.dispose()


@pytest.fixture
def performance_session_factory(
    performance_engine: AsyncEngine,
) -> Callable[[], AsyncSession]:
    """Create a test database session factory for performance tests."""
    return async_sessionmaker(
        performance_engine, class_=AsyncSession, expire_on_commit=False
    )


@pytest.fixture
def performance_app_context() -> AppContext:
    """Create a test app context for performance tests."""
    with tempfile.TemporaryDirectory() as data_dir:
        return AppContext(
            data_dir=Path(data_dir),
            db_url="sqlite+aiosqlite:///:memory:",
            log_level="INFO",
            disable_telemetry=True,
            _env_file=None,  # type: ignore[call-arg]
        )


@pytest.mark.asyncio
async def test_clone_and_scan_performance_helix_repository(
    performance_session_factory: Callable[[], AsyncSession],
) -> None:
    """Measure clone and scan performance on helixml/helix repository.

    This is a regression test to track performance over time. The helixml/helix
    repository is a medium-sized repository that provides a good benchmark.

    Baseline performance (as of 2025-11-03):
    - Repository: https://github.com/helixml/helix
    - Commits: ~19,000
    - Branches: ~280
    - Tags: ~340
    - Clone time: ~4 seconds
    - Scan time: ~23 seconds
    - Database save: ~1 second
    - Total time: ~28 seconds
    """
    # Repository to test
    repo_url = AnyUrl("https://github.com/helixml/helix")

    # Setup dependencies
    with tempfile.TemporaryDirectory() as tmp_clone_dir:
        clone_dir = Path(tmp_clone_dir)
        git_adapter = GitPythonAdapter()
        cloner = RepositoryCloner(git_adapter, clone_dir)
        scanner = GitRepositoryScanner(git_adapter)
        create_noop_operation()

        repo_repository = create_git_repo_repository(
            session_factory=performance_session_factory
        )
        git_commit_repository = create_git_commit_repository(
            session_factory=performance_session_factory
        )

        # Create repository entity
        repo = GitRepoFactory.create_from_remote_uri(repo_url)
        repo = await repo_repository.save(repo)
        assert repo.id is not None

        # Measure clone performance

        clone_start = time.perf_counter()
        cloned_path = await cloner.clone_repository(repo_url)
        clone_end = time.perf_counter()
        clone_duration = clone_end - clone_start


        # Update repository with cloned path
        repo.cloned_path = cloned_path
        await repo_repository.save(repo)

        # Measure scan performance
        scan_start = time.perf_counter()
        scan_result = await scanner.scan_repository(cloned_path, repo.id)
        scan_end = time.perf_counter()
        scan_duration = scan_end - scan_start


        # Save commits to test database operations
        db_save_start = time.perf_counter()
        await git_commit_repository.save_bulk(scan_result.all_commits)
        db_save_end = time.perf_counter()
        db_save_duration = db_save_end - db_save_start


        # Summary
        clone_duration + scan_duration + db_save_duration

        # Assertions to ensure the test actually worked
        assert scan_result.all_commits, "Should have found commits"
        assert scan_result.branches, "Should have found branches"
        assert clone_duration > 0, "Clone should take measurable time"
        assert scan_duration > 0, "Scan should take measurable time"

        # Performance expectations (these are baseline values, adjust as needed)
        # These are intentionally generous to avoid flaky tests due to network
        # variability and different machine performance
        assert clone_duration < 300, "Clone should complete within 5 minutes"
        assert scan_duration < 180, "Scan should complete within 3 minutes"
        assert (
            db_save_duration < 60
        ), "Database save should complete within 1 minute"
