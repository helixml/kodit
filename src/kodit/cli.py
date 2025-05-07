"""Command line interface for kodit."""

import os

import click
import structlog
import uvicorn
from dotenv import dotenv_values
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.database import configure_database, with_session
from kodit.logging import LogFormat, configure_logging, disable_posthog, log_event
from kodit.models import Source

env_vars = dict(dotenv_values())
os.environ.update(env_vars)


@click.group(context_settings={"auto_envvar_prefix": "KODIT", "show_default": True})
@click.option("--log-level", default="INFO", help="Log level")
@click.option("--log-format", default=LogFormat.PRETTY, help="Log format")
@click.option("--disable-telemetry", is_flag=True, help="Disable telemetry")
def cli(
    log_level: str,
    log_format: LogFormat,
    disable_telemetry: bool,  # noqa: FBT001
) -> None:
    """kodit CLI - Code indexing for better AI code generation."""  # noqa: D403
    configure_logging(log_level, log_format)
    if disable_telemetry:
        disable_posthog()
    # Initialize database
    configure_database()


@cli.group()
def sources() -> None:
    """Manage code sources."""


@sources.command(name="list")
@with_session
async def list_sources(session: AsyncSession) -> None:
    """List all code sources."""
    log = structlog.get_logger(__name__)
    result = await session.execute(select(Source))
    sources = result.scalars().all()

    if not sources:
        log.info("No sources found")
        return

    for source in sources:
        log.info(
            "Source found",
            id=source.id,
            name=source.name,
            path=source.path,
            created_at=source.created_at,
            updated_at=source.updated_at,
        )


@sources.command()
@click.argument("name")
@click.argument("path")
@with_session
async def add(session: AsyncSession, name: str, path: str) -> None:
    """Add a new code source.

    NAME: The name of the source
    PATH: The path to the source directory
    """
    log = structlog.get_logger(__name__)
    source = Source(name=name, path=path)
    session.add(source)
    await session.commit()
    log.info(
        "Source added",
        id=source.id,
        name=source.name,
        path=source.path,
        created_at=source.created_at,
    )


@cli.command()
@click.option("--host", default="127.0.0.1", help="Host to bind the server to")
@click.option("--port", default=8080, help="Port to bind the server to")
@click.option("--reload", is_flag=True, help="Enable auto-reload for development")
def serve(
    host: str,
    port: int,
    reload: bool,  # noqa: FBT001
) -> None:
    """Start the kodit server, which hosts the MCP server and the kodit API."""
    log = structlog.get_logger(__name__)
    log.info("Starting kodit server", host=host, port=port, reload=reload)
    log_event("kodit_server_started")
    uvicorn.run(
        "kodit.app:app",
        host=host,
        port=port,
        reload=reload,
        log_config=None,  # Setting to None forces uvicorn to use our structlog setup
        access_log=False,  # Using own middleware for access logging
    )


@cli.command()
def version() -> None:
    """Show the version of kodit."""
    try:
        from kodit import _version
    except ImportError:
        print("unknown, try running `uv build`, which is what happens in ci")  # noqa: T201
    else:
        print(_version.version)  # noqa: T201


if __name__ == "__main__":
    cli()
