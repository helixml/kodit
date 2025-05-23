"""Global configuration for the kodit project."""

import asyncio
from collections.abc import Callable, Coroutine
from functools import wraps
from pathlib import Path
from typing import Any, TypeVar

import click
from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict

from kodit.database import Database
from kodit.embedding.embedding import TINY

DEFAULT_BASE_DIR = Path.home() / ".kodit"
DEFAULT_DB_URL = f"sqlite+aiosqlite:///{DEFAULT_BASE_DIR}/kodit.db"
DEFAULT_LOG_LEVEL = "INFO"
DEFAULT_LOG_FORMAT = "pretty"
DEFAULT_DISABLE_TELEMETRY = False
DEFAULT_EMBEDDING_MODEL_NAME = TINY
T = TypeVar("T")


class AppContext(BaseSettings):
    """Global context for the kodit project. Provides a shared state for the app."""

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

    async def get_db(self, *, run_migrations: bool = True) -> Database:
        """Get the database."""
        if self._db is None:
            self._db = Database(self.db_url)
        if run_migrations:
            await self._db.run_migrations(self.db_url)
        return self._db


with_app_context = click.make_pass_decorator(AppContext)

T = TypeVar("T")


def wrap_async(f: Callable[..., Coroutine[Any, Any, T]]) -> Callable[..., T]:
    """Decorate async Click commands.

    This decorator wraps an async function to run it with asyncio.run().
    It should be used after the Click command decorator.

    Example:
        @cli.command()
        @wrap_async
        async def my_command():
            ...

    """

    @wraps(f)
    def wrapper(*args: Any, **kwargs: Any) -> T:
        return asyncio.run(f(*args, **kwargs))

    return wrapper


def with_session(f: Callable[..., Coroutine[Any, Any, T]]) -> Callable[..., T]:
    """Provide a database session to CLI commands."""

    @wraps(f)
    @with_app_context
    @wrap_async
    async def wrapper(app_context: AppContext, *args: Any, **kwargs: Any) -> T:
        db = await app_context.get_db()
        async with db.session_factory() as session:
            return await f(session, *args, **kwargs)

    return wrapper
