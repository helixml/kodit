"""Test configuration and fixtures."""

from collections.abc import AsyncGenerator
from pathlib import Path
import tempfile
from typing import Generator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncEngine, AsyncSession, create_async_engine
from sqlalchemy.orm import sessionmaker

from kodit.config import AppContext
from kodit.database import Base


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
    async_session = sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)

    async with async_session() as session:
        yield session
        await session.rollback()


@pytest.fixture
def app_context() -> Generator[AppContext, None, None]:
    """Create a test app context."""
    with tempfile.TemporaryDirectory() as data_dir:
        app_context = AppContext(
            data_dir=Path(data_dir),
            db_url="sqlite+aiosqlite:///:memory:",
            log_level="DEBUG",
            log_format="json",
            disable_telemetry=True,
        )
        yield app_context
