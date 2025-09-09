"""Database configuration for kodit."""

from collections.abc import Callable
from pathlib import Path
from typing import Any

import structlog
from alembic import command
from alembic.config import Config as AlembicConfig
from sqlalchemy import event
from sqlalchemy.ext.asyncio import (
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from kodit import migrations


class Database:
    """Database class for kodit."""

    def __init__(self, db_url: str) -> None:
        """Initialize the database."""
        self.log = structlog.get_logger(__name__)

        # Configure SQLite-specific connection arguments to prevent locking issues
        connect_args = {}
        if "sqlite" in db_url.lower():
            connect_args = {
                "timeout": 20,  # 20 second timeout for database operations
                "check_same_thread": False,  # Allow use from different threads
            }

        self.db_engine = create_async_engine(
            db_url,
            echo=False,
            connect_args=connect_args,
        )

        # Configure SQLite pragmas for better concurrency and performance
        if "sqlite" in db_url.lower():

            @event.listens_for(self.db_engine.sync_engine, "connect")
            def set_sqlite_pragma(
                dbapi_connection: Any, connection_record: Any
            ) -> None:
                del (
                    connection_record
                )  # Unused but required by SQLAlchemy event interface
                cursor = dbapi_connection.cursor()
                # Enable WAL mode for better concurrency
                cursor.execute("PRAGMA journal_mode=WAL")
                # Set busy timeout to prevent immediate locking failures
                cursor.execute("PRAGMA busy_timeout=20000")
                # Enable foreign key constraints
                cursor.execute("PRAGMA foreign_keys=ON")
                # Optimize for speed over safety (acceptable for indexing workloads)
                cursor.execute("PRAGMA synchronous=NORMAL")
                cursor.close()

        self.db_session_factory = async_sessionmaker(
            self.db_engine,
            class_=AsyncSession,
            expire_on_commit=False,
        )

    @property
    def session_factory(self) -> Callable[[], AsyncSession]:
        """Get the session factory."""
        return self.db_session_factory

    async def run_migrations(self, db_url: str) -> None:
        """Run any pending migrations."""
        # Create Alembic configuration and run migrations
        alembic_cfg = AlembicConfig()
        alembic_cfg.set_main_option(
            "script_location", str(Path(migrations.__file__).parent)
        )
        alembic_cfg.set_main_option("sqlalchemy.url", db_url)
        self.log.debug("Running migrations", db_url=db_url)

        async with self.db_engine.begin() as conn:
            await conn.run_sync(self.run_upgrade, alembic_cfg)

    def run_upgrade(self, connection, cfg) -> None:  # noqa: ANN001
        """Make sure the database is up to date."""
        cfg.attributes["connection"] = connection
        command.upgrade(cfg, "head")

    async def close(self) -> None:
        """Close the database."""
        await self.db_engine.dispose()
