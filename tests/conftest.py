"""Test configuration and fixtures."""

import tempfile
from collections.abc import AsyncGenerator, Generator
from pathlib import Path
from unittest.mock import patch

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from kodit.config import AppContext, LogFormat

# Need to import these models to create the tables
from kodit.infrastructure.sqlalchemy.entities import (
    Base,
)


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
