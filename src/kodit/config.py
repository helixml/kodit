"""Global configuration for the kodit project."""

from __future__ import annotations

import asyncio
from functools import wraps
from pathlib import Path
from typing import TYPE_CHECKING, Any, Literal, TypeVar

import click
from pydantic import BaseModel, Field, field_validator
from pydantic_settings import (
    BaseSettings,
    EnvSettingsSource,
    PydanticBaseSettingsSource,
    SettingsConfigDict,
)

if TYPE_CHECKING:
    from collections.abc import Callable, Coroutine


from kodit.database import Database

DEFAULT_BASE_DIR = Path.home() / ".kodit"
DEFAULT_LOG_LEVEL = "INFO"
DEFAULT_LOG_FORMAT = "pretty"
DEFAULT_DISABLE_TELEMETRY = False
T = TypeVar("T")

EndpointType = Literal["openai"]


class Endpoint(BaseModel):
    """Endpoint provides configuration for an AI service."""

    type: EndpointType | None = None
    base_url: str | None = None
    model: str | None = None
    api_key: str | None = None


class Search(BaseModel):
    """Search configuration."""

    provider: Literal["sqlite", "vectorchord"] = Field(default="sqlite")


class AutoIndexingSource(BaseModel):
    """Configuration for a single auto-indexing source."""

    uri: str = Field(description="URI of the source to index (git URL or local path)")


class AutoIndexingConfig(BaseModel):
    """Configuration for auto-indexing."""

    sources: list[AutoIndexingSource] = Field(
        default_factory=list, description="List of sources to auto-index"
    )

    @field_validator("sources", mode="before")
    @classmethod
    def parse_sources(cls, v: Any) -> Any:
        """Parse sources from environment variables or other formats."""
        if v is None:
            return []
        if isinstance(v, list):
            return v
        if isinstance(v, dict):
            # Handle case where env vars are numbered keys like {'0': {'uri': '...'}}
            sources = []
            i = 0
            while str(i) in v:
                source_data = v[str(i)]
                if isinstance(source_data, dict) and "uri" in source_data:
                    sources.append(AutoIndexingSource(uri=source_data["uri"]))
                i += 1
            return sources
        return v


class PeriodicSyncConfig(BaseModel):
    """Configuration for periodic/scheduled syncing."""

    enabled: bool = Field(default=True, description="Enable periodic sync")
    interval_seconds: float = Field(
        default=1800, description="Interval between automatic syncs in seconds"
    )
    retry_attempts: int = Field(
        default=3, description="Number of retry attempts for failed syncs"
    )


class CustomAutoIndexingEnvSource(EnvSettingsSource):
    """Custom environment source for parsing AutoIndexingConfig."""

    def __call__(self) -> dict[str, Any]:
        """Load settings from env vars with custom auto-indexing parsing."""
        d: dict[str, Any] = {}

        # First get the standard env vars
        env_vars = super().__call__()
        d.update(env_vars)

        # Custom parsing for auto-indexing sources
        auto_indexing_sources = []
        i = 0
        while True:
            # Note: env_vars keys are lowercase due to Pydantic Settings normalization
            uri_key = f"auto_indexing_sources_{i}_uri"
            if uri_key in self.env_vars:
                uri_value = self.env_vars[uri_key]
                auto_indexing_sources.append({"uri": uri_value})
                i += 1
            else:
                break

        if auto_indexing_sources:
            d["auto_indexing"] = {"sources": auto_indexing_sources}

        return d


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

    @classmethod
    def settings_customise_sources(
        cls,
        settings_cls: type[BaseSettings],
        init_settings: PydanticBaseSettingsSource,
        env_settings: PydanticBaseSettingsSource,  # noqa: ARG003
        dotenv_settings: PydanticBaseSettingsSource,
        file_secret_settings: PydanticBaseSettingsSource,
    ) -> tuple[PydanticBaseSettingsSource, ...]:
        """Customize settings sources to use custom auto-indexing parsing."""
        custom_env_settings = CustomAutoIndexingEnvSource(
            settings_cls,
            env_nested_delimiter=settings_cls.model_config.get("env_nested_delimiter"),
            env_ignore_empty=settings_cls.model_config.get("env_ignore_empty", False),
            env_parse_none_str=settings_cls.model_config.get("env_parse_none_str", ""),
            env_parse_enums=settings_cls.model_config.get("env_parse_enums", None),
        )
        return (
            init_settings,
            custom_env_settings,
            dotenv_settings,
            file_secret_settings,
        )

    data_dir: Path = Field(default=DEFAULT_BASE_DIR)
    db_url: str = Field(
        default_factory=lambda data: f"sqlite+aiosqlite:///{data['data_dir']}/kodit.db"
    )
    log_level: str = Field(default=DEFAULT_LOG_LEVEL)
    log_format: str = Field(default=DEFAULT_LOG_FORMAT)
    disable_telemetry: bool = Field(default=DEFAULT_DISABLE_TELEMETRY)
    default_endpoint: Endpoint | None = Field(
        default=None,
        description=(
            "Default endpoint to use for all AI interactions "
            "(can be overridden by task-specific configuration)."
        ),
    )
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
    auto_indexing: AutoIndexingConfig | None = Field(
        default=AutoIndexingConfig(), description="Auto-indexing configuration"
    )
    periodic_sync: PeriodicSyncConfig = Field(
        default=PeriodicSyncConfig(), description="Periodic sync configuration"
    )
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
