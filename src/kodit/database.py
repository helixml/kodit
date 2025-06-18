"""Database configuration for kodit."""

from pathlib import Path

import structlog
from alembic import command
from alembic.config import Config as AlembicConfig
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
        self.db_engine = create_async_engine(db_url, echo=False)
        self.db_session_factory = async_sessionmaker(
            self.db_engine,
            class_=AsyncSession,
            expire_on_commit=False,
        )

    @property
    def session_factory(self) -> async_sessionmaker[AsyncSession]:
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
