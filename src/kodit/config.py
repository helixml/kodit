"""Global configuration for the kodit project."""

import asyncio
from collections.abc import Callable
from functools import wraps
from pathlib import Path
from typing import Any, TypeVar

import click
from pydantic import Field, SkipValidation
from pydantic_settings import BaseSettings, SettingsConfigDict

from kodit.database import Database
from kodit.logging import LogFormat, configure_logging, disable_posthog

DEFAULT_BASE_DIR = Path.home() / ".kodit"
DEFAULT_DB_URL = f"sqlite+aiosqlite:///{DEFAULT_BASE_DIR}/kodit.db"


class Config(BaseSettings):
    """Global configuration for the kodit project."""

    model_config = SettingsConfigDict(extra="allow")

    data_dir: Path = Field(default=DEFAULT_BASE_DIR)
    db_url: str = Field(default=DEFAULT_DB_URL)
    log_level: str = Field(default="INFO")
    log_format: SkipValidation[LogFormat] = Field(default=LogFormat.PRETTY)
    disable_telemetry: bool = Field(default=False)
    _db: Database | None = None

    def model_post_init(self, _: Any) -> None:
        """Post-initialization configuration."""
        configure_logging(self.log_level, self.log_format)
        if self.disable_telemetry:
            disable_posthog()

        Path(self.data_dir).mkdir(exist_ok=True)
        self._db = Database(self.db_url)

    def get_clone_dir(self) -> Path:
        """Get the clone directory."""
        return self.data_dir / "clones"

    def get_db(self) -> Database:
        """Get the database."""
        if self._db is None:
            msg = "Database not initialized"
            raise RuntimeError(msg)
        return self._db


# Global config instance for mcp Apps
config = Config()

pass_config = click.make_pass_decorator(Config, ensure=True)


T = TypeVar("T")


def with_session(func: Callable[..., T]) -> Callable[..., T]:
    """Provide an async session to CLI commands."""
    db = config.get_db()

    @wraps(func)
    def wrapper(*args: Any, **kwargs: Any) -> T:
        async def _run() -> T:
            async with db.get_session() as session:
                return await func(session, *args, **kwargs)

        return asyncio.run(_run())

    return wrapper
