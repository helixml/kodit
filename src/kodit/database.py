"""Database configuration for kodit."""

from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from pathlib import Path

from alembic import command
from alembic.config import Config as AlembicConfig
from sqlalchemy import DateTime
from sqlalchemy.ext.asyncio import (
    AsyncAttrs,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column

from kodit import alembic


class Base(AsyncAttrs, DeclarativeBase):
    """Base class for all models."""


class CommonMixin:
    """Common mixin for all models."""

    id: Mapped[int] = mapped_column(primary_key=True, autoincrement=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime, default=lambda: datetime.now(UTC)
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime, default=lambda: datetime.now(UTC), onupdate=lambda: datetime.now(UTC)
    )


class Database:
    """Database class for kodit."""

    def __init__(self, db_url: str) -> None:
        """Initialize the database."""
        db_engine = create_async_engine(db_url, echo=False)
        self._configure_database(db_url)
        self.db_session_factory = async_sessionmaker(
            db_engine,
            class_=AsyncSession,
            expire_on_commit=False,
        )

    @asynccontextmanager
    async def get_session(self) -> AsyncGenerator[AsyncSession, None]:
        """Get a database session."""
        async with self.db_session_factory() as session:
            try:
                yield session
            finally:
                await session.close()

    def _configure_database(self, db_url: str) -> None:
        """Run any pending migrations."""
        # Create Alembic configuration and run migrations
        alembic_cfg = AlembicConfig()
        alembic_cfg.set_main_option(
            "script_location", str(Path(alembic.__file__).parent)
        )
        alembic_cfg.set_main_option("sqlalchemy.url", db_url)
        command.upgrade(alembic_cfg, "head")
