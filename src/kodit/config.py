"""Global configuration for the kodit project."""

import asyncio
from collections.abc import Callable
from dataclasses import dataclass
from functools import wraps
from pathlib import Path
from typing import Any, TypeVar

import click

from kodit.database import Database
from kodit.logging import LogFormat, configure_logging, disable_posthog


@dataclass
class Config:
    """Global configuration for the kodit project."""

    data_dir: Path
    db_url: str | None
    log_level: str
    log_format: LogFormat
    disable_telemetry: bool

    def __post_init__(self) -> None:
        """Post-initialization configuration."""
        configure_logging(self.log_level, self.log_format)
        if self.disable_telemetry:
            disable_posthog()

        Path(self.data_dir).mkdir(exist_ok=True)
        if not self.db_url:
            self.db_url = f"sqlite+aiosqlite:///{self.data_dir}/kodit.db"
        self.db = Database(self.db_url)

    def get_clone_dir(self) -> Path:
        """Get the clone directory."""
        return self.data_dir / "clones"


pass_config = click.make_pass_decorator(Config, ensure=True)


T = TypeVar("T")


def with_session(func: Callable[..., T]) -> Callable[..., T]:
    """Provide an async session to CLI commands."""

    @wraps(func)
    @pass_config
    def wrapper(config: Config, *args: Any, **kwargs: Any) -> T:
        async def _run() -> T:
            async with config.db.get_session() as session:
                return await func(session, *args, **kwargs)

        return asyncio.run(_run())

    return wrapper
