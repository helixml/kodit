"""Database configuration for kodit."""

import asyncio
from collections.abc import AsyncGenerator, Callable
from functools import wraps
from pathlib import Path
from typing import Any, TypeVar

from alembic import command
from alembic.config import Config
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy.orm import DeclarativeBase

# Constants
DATA_DIR = Path.home() / ".kodit"
DB_URL = f"sqlite+aiosqlite:///{DATA_DIR}/kodit.db"

# Create data directory if it doesn't exist
DATA_DIR.mkdir(exist_ok=True)

# Create async engine with file-based SQLite
engine = create_async_engine(DB_URL, echo=False)

# Create async session factory
async_session_factory = async_sessionmaker(
    engine,
    class_=AsyncSession,
    expire_on_commit=False,
)


class Base(DeclarativeBase):
    """Base class for all models."""


async def get_session() -> AsyncGenerator[AsyncSession, None]:
    """Get a database session."""
    async with async_session_factory() as session:
        try:
            yield session
        finally:
            await session.close()


T = TypeVar("T")


def with_session(func: Callable[..., T]) -> Callable[..., T]:
    """Provide an async session to CLI commands."""

    @wraps(func)
    def wrapper(*args: Any, **kwargs: Any) -> T:
        async def _run() -> T:
            async with async_session_factory() as session:
                return await func(session, *args, **kwargs)

        return asyncio.run(_run())

    return wrapper


def configure_database() -> None:
    """Configure the database by initializing it and running any pending migrations."""
    # Create Alembic configuration and run migrations
    alembic_cfg = Config("alembic.ini")
    command.upgrade(alembic_cfg, "head")
