"""Global configuration for the kodit project."""

import asyncio
from collections.abc import Callable
from functools import wraps
from pathlib import Path
from typing import Any, TypeVar

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict

from kodit.database import Database

DEFAULT_BASE_DIR = Path.home() / ".kodit"
DEFAULT_DB_URL = f"sqlite+aiosqlite:///{DEFAULT_BASE_DIR}/kodit.db"
DEFAULT_LOG_LEVEL = "INFO"
DEFAULT_LOG_FORMAT = "pretty"
DEFAULT_DISABLE_TELEMETRY = False
T = TypeVar("T")


class Config(BaseSettings):
    """Global configuration for the kodit project."""

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

    data_dir: Path = Field(default=DEFAULT_BASE_DIR)
    db_url: str = Field(default=DEFAULT_DB_URL)
    log_level: str = Field(default=DEFAULT_LOG_LEVEL)
    log_format: str = Field(default=DEFAULT_LOG_FORMAT)
    disable_telemetry: bool = Field(default=DEFAULT_DISABLE_TELEMETRY)
    _db: Database | None = None

    def model_post_init(self, _: Any) -> None:
        """Post-initialization hook."""
        # Call this to ensure the data dir exists for the default db location
        self.get_data_dir()

    def get_data_dir(self) -> Path:
        """Get the data directory."""
        self.data_dir.mkdir(parents=True, exist_ok=True)
        return self.data_dir

    def get_clone_dir(self) -> Path:
        """Get the clone directory."""
        clone_dir = self.get_data_dir() / "clones"
        clone_dir.mkdir(parents=True, exist_ok=True)
        return clone_dir

    def get_db(self, *, run_migrations: bool = True) -> Database:
        """Get the database."""
        if self._db is None:
            self._db = Database(self.db_url, run_migrations=run_migrations)
        return self._db


# Global config instance for mcp Apps
config = None


def get_config(env_file: str | None = None) -> Config:
    """Get the global config instance."""
    global config  # noqa: PLW0603
    if config is None:
        config = Config(_env_file=env_file)
    return config


def reset_config() -> None:
    """Reset the global config instance."""
    global config  # noqa: PLW0603
    config = None


def with_session(func: Callable[..., T]) -> Callable[..., T]:
    """Provide an async session to CLI commands."""

    @wraps(func)
    def wrapper(*args: Any, **kwargs: Any) -> T:
        # Create DB connection before starting event loop
        db = get_config().get_db()

        async def _run() -> T:
            async with db.get_session() as session:
                return await func(session, *args, **kwargs)

        return asyncio.run(_run())

    return wrapper
