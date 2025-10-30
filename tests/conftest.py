"""Test configuration and fixtures."""

import tempfile
from collections.abc import AsyncGenerator, Callable, Generator
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import patch

import pytest
from pydantic import AnyUrl
from sqlalchemy import text
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from kodit.config import AppContext, LogFormat
from kodit.domain.entities.git import GitCommit, GitFile, GitRepo

# Need to import these models to create the tables
from kodit.infrastructure.sqlalchemy.entities import (
    Base,
)
from kodit.infrastructure.sqlalchemy.unit_of_work import SqlAlchemyUnitOfWork


@pytest.fixture
async def engine() -> AsyncGenerator[AsyncEngine, None]:
    """Create a test database engine."""
    # Use SQLite in-memory database for testing
    engine = create_async_engine(
        "sqlite+aiosqlite:///:memory:",
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
async def session(engine: AsyncEngine) -> AsyncGenerator[AsyncSession, None]:
    """Create a test database session."""
    async_session = async_sessionmaker(
        engine, class_=AsyncSession, expire_on_commit=False
    )

    async with async_session() as session:
        yield session
        await session.rollback()


@pytest.fixture
def app_context() -> Generator[AppContext, None, None]:
    """Create a test app context."""
    import os

    # Create a minimal environment with only essential env vars
    essential_prefixes = (
        "PATH",
        "HOME",
        "USER",
        "PWD",
        "LANG",
        "LC_",
        "TERM",
        "SHELL",
        "TMPDIR",
    )
    minimal_env = {
        key: value
        for key, value in os.environ.items()
        if key.startswith(essential_prefixes)
    }

    with tempfile.TemporaryDirectory() as data_dir:
        # Patch os.environ to use minimal environment during AppContext creation
        with patch.dict(os.environ, minimal_env, clear=True):
            app_context = AppContext(
                data_dir=Path(data_dir),
                db_url="sqlite+aiosqlite:///:memory:",
                log_level="DEBUG",
                log_format=LogFormat.JSON,
                disable_telemetry=True,
                _env_file=None,  # type: ignore[call-arg]
            )
        yield app_context


@pytest.fixture
def session_factory(engine: AsyncEngine) -> Callable[[], AsyncSession]:
    """Create a test database session factory."""
    return async_sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)


@pytest.fixture
def unit_of_work(session_factory: Callable[[], AsyncSession]) -> SqlAlchemyUnitOfWork:
    """Create a test unit of work."""
    return SqlAlchemyUnitOfWork(session_factory)


# Shared fixtures for deletion tests


@pytest.fixture
def sample_git_file() -> GitFile:
    """Create a sample git file for deletion tests."""
    return GitFile(
        created_at=datetime.now(UTC),
        blob_sha="file_sha_123",
        commit_sha="commit_sha_456",
        path="src/main.py",
        mime_type="text/x-python",
        size=1024,
        extension="py",
    )


@pytest.fixture
def sample_git_commit() -> GitCommit:
    """Create a sample git commit for deletion tests."""
    return GitCommit(
        created_at=datetime.now(UTC),
        commit_sha="commit_sha_456",
        repo_id=1,
        date=datetime.now(UTC),
        message="Test commit",
        parent_commit_sha=None,
        author="test@example.com",
    )


@pytest.fixture
def sample_git_repo() -> GitRepo:
    """Create a sample git repository for deletion tests."""
    return GitRepo(
        id=None,
        created_at=datetime.now(UTC),
        sanitized_remote_uri=AnyUrl("https://github.com/test/repo"),
        remote_uri=AnyUrl("https://github.com/test/repo.git"),
        num_commits=1,
        num_branches=1,
        num_tags=1,
    )
