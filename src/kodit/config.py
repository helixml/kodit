"""Global configuration for the kodit project."""

from __future__ import annotations

import asyncio
from datetime import timedelta
from enum import Enum
from functools import wraps
from pathlib import Path
from typing import TYPE_CHECKING, Annotated, Any, Literal, TypeVar

import click
import structlog
from pydantic import BaseModel, Field, field_validator
from pydantic_settings import (
    BaseSettings,
    NoDecode,
    SettingsConfigDict,
)

from kodit.database import Database

if TYPE_CHECKING:
    from collections.abc import Callable, Coroutine


class LogFormat(Enum):
    """The format of the log output."""

    PRETTY = "pretty"
    JSON = "json"


DEFAULT_BASE_DIR = Path.home() / ".kodit"
DEFAULT_LOG_LEVEL = "INFO"
DEFAULT_LOG_FORMAT = LogFormat.PRETTY
DEFAULT_DISABLE_TELEMETRY = False
T = TypeVar("T")


class ReportingConfig(BaseModel):
    """Reporting configuration."""

    log_time_interval: timedelta = Field(
        default=timedelta(seconds=5),
        description="Time interval to log progress in seconds",
    )


class Endpoint(BaseModel):
    """Endpoint provides configuration for an AI service."""

    base_url: str | None = Field(
        default=None,
        description="Base URL for the endpoint (e.g. 'https://app.helix.ml/v1')",
    )
    model: str | None = Field(
        default=None,
        description="Model to use for the endpoint in litellm format (e.g. 'openai/text-embedding-3-small' or 'hosted_vllm/Qwen/Qwen3-8B')",  # noqa: E501
    )
    api_key: str | None = Field(
        default=None,
        description="API key for the endpoint",
    )
    num_parallel_tasks: int = Field(
        default=10,
        description="Number of parallel tasks to use for the endpoint",
    )
    socket_path: str | None = Field(
        default=None,
        description="Unix socket path for local communication (e.g., /tmp/openai.sock)",
    )
    timeout: float = Field(
        default=60,
        description="Request timeout in seconds",
    )
    max_retries: int = Field(
        default=5,
        description="Maximum number of retries for the endpoint",
    )
    initial_delay: float = Field(
        default=2.0,
        description="Initial delay in seconds for the endpoint",
    )
    backoff_factor: float = Field(
        default=2.0,
        description="Backoff factor for the endpoint",
    )
    extra_params: dict[str, Any] | None = Field(
        default=None,
        description="Extra provider-specific non-secret parameters for LiteLLM",
    )
    max_tokens: int = Field(
        default=8000,  # Reasonable default (with headroom) for most models.
        description="Conservative token limit for the embedding model",
    )


DEFAULT_NUM_PARALLEL_TASKS = 10  # Semaphore limit for concurrent requests


class Search(BaseModel):
    """Search configuration."""

    provider: Literal["sqlite", "vectorchord"] = Field(default="sqlite")


class PeriodicSyncConfig(BaseModel):
    """Configuration for periodic/scheduled syncing."""

    enabled: bool = Field(default=True, description="Enable periodic sync")
    interval_seconds: float = Field(
        default=1800, description="Interval between periodic syncs in seconds"
    )
    retry_attempts: int = Field(
        default=3, description="Number of retry attempts for failed syncs"
    )


class RemoteConfig(BaseModel):
    """Configuration for remote server connection."""

    server_url: str | None = Field(default=None, description="Remote Kodit server URL")
    api_key: str | None = Field(default=None, description="API key for authentication")
    timeout: float = Field(default=30.0, description="Request timeout in seconds")
    max_retries: int = Field(default=3, description="Maximum retry attempts")
    verify_ssl: bool = Field(default=True, description="Verify SSL certificates")


class AppContext(BaseSettings):
    """Global context for the kodit project. Provides a shared state for the app."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        env_nested_delimiter="_",
        env_nested_max_split=1,
        nested_model_default_partial_update=True,
        extra="ignore",
    )

    data_dir: Path = Field(default=DEFAULT_BASE_DIR)
    db_url: str = Field(
        default_factory=lambda data: f"sqlite+aiosqlite:///{data['data_dir']}/kodit.db"
    )
    log_level: str = Field(default=DEFAULT_LOG_LEVEL)
    log_format: LogFormat = Field(default=DEFAULT_LOG_FORMAT)
    disable_telemetry: bool = Field(default=DEFAULT_DISABLE_TELEMETRY)
    embedding_endpoint: Endpoint | None = Field(
        default=None,
        description="Endpoint to use for embedding.",
    )
    enrichment_endpoint: Endpoint | None = Field(
        default=None,
        description="Endpoint to use for enrichment.",
    )
    default_search: Search = Field(
        default=Search(),
    )
    periodic_sync: PeriodicSyncConfig = Field(
        default=PeriodicSyncConfig(), description="Periodic sync configuration"
    )
    api_keys: Annotated[list[str], NoDecode] = Field(
        default_factory=list,
        description="Comma-separated list of valid API keys (e.g. 'key1,key2')",
    )
    remote: RemoteConfig = Field(
        default_factory=RemoteConfig, description="Remote server configuration"
    )
    reporting: ReportingConfig = Field(
        default=ReportingConfig(), description="Reporting configuration"
    )

    @field_validator("api_keys", mode="before")
    @classmethod
    def parse_api_keys(cls, v: Any) -> list[str]:
        """Parse API keys from CSV format."""
        if v is None:
            return []
        if isinstance(v, list):
            return v
        if isinstance(v, str):
            # Split by comma and strip whitespace
            return [key.strip() for key in v.strip().split(",") if key.strip()]
        return v

    _db: Database | None = None
    _log = structlog.get_logger(__name__)

    def model_post_init(self, _: Any) -> None:
        """Post-initialization hook."""
        # Call this to ensure the data dir exists for the default db location
        self.get_data_dir()

    @property
    def is_remote(self) -> bool:
        """Check if running in remote mode."""
        return self.remote.server_url is not None

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
            self._db = await self.new_db(run_migrations=run_migrations)
        return self._db

    async def new_db(self, *, run_migrations: bool = True) -> Database:
        """Get a completely fresh connection to a database.

        This is required when running tasks in a thread pool.
        """
        db = Database(self.db_url)
        if run_migrations:
            await db.run_migrations(self.db_url)
        return db


with_app_context = click.make_pass_decorator(AppContext)


def wrap_async[T](f: Callable[..., Coroutine[Any, Any, T]]) -> Callable[..., T]:
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


def with_session[T](f: Callable[..., Coroutine[Any, Any, T]]) -> Callable[..., T]:
    """Provide a database session to CLI commands."""

    @wraps(f)
    @with_app_context
    @wrap_async
    async def wrapper(app_context: AppContext, *args: Any, **kwargs: Any) -> T:
        db = await app_context.get_db()
        async with db.session_factory() as session:
            return await f(session, *args, **kwargs)

    return wrapper
