"""Command line interface for kodit."""

# TODO:
# 4. Could do with some more testing on the new config object. E.g. are the env vars the same when using cli vs server?
# 5. some more finesse
# 6. some better e2e tests (partially implemented)

import os
import signal
from pathlib import Path
from typing import Any

import click
import structlog
import uvicorn
from dotenv import dotenv_values
from pytable_formatter import Table
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import DEFAULT_DB_URL, config, with_session
from kodit.indexing.repository import IndexRepository
from kodit.indexing.service import IndexService
from kodit.logging import LogFormat, log_event
from kodit.retreival.repository import RetrievalRepository
from kodit.retreival.service import RetrievalRequest, RetrievalService
from kodit.sources.repository import SourceRepository
from kodit.sources.service import SourceService

env_vars = dict(dotenv_values())
os.environ.update(env_vars)


@click.group(context_settings={"auto_envvar_prefix": "KODIT", "show_default": True})
@click.option("--log-level", default="INFO", help="Log level")
@click.option("--log-format", default=LogFormat.PRETTY, help="Log format")
@click.option("--disable-telemetry", is_flag=True, help="Disable telemetry")
@click.option(
    "--db-url",
    default=DEFAULT_DB_URL,
    help="Database URL",
)
@click.option(
    "--data-dir",
    default=str(Path.home() / ".kodit"),
    help="Directory to store kodit data",
)
@click.pass_context
def cli(
    ctx: click.Context,
    log_level: str,
    log_format: LogFormat,
    disable_telemetry: bool,  # noqa: FBT001
    db_url: str,
    data_dir: str,
) -> None:
    """kodit CLI - Code indexing for better AI code generation."""  # noqa: D403
    # Override global config with cli args
    config.data_dir = Path(data_dir)
    config.db_url = db_url
    config.log_level = log_level
    config.log_format = log_format
    config.disable_telemetry = disable_telemetry


@cli.group()
def sources() -> None:
    """Manage code sources."""


@sources.command(name="list")
@with_session
async def list_sources(session: AsyncSession) -> None:
    """List all code sources."""
    repository = SourceRepository(session)
    service = SourceService(config.get_clone_dir(), repository)
    sources = await service.list_sources()

    # Define headers and data
    headers = ["ID", "Created At", "URI"]
    data = [[source.id, source.created_at, source.uri] for source in sources]

    # Create and display the table
    table = Table(headers=headers, data=data)
    click.echo(table)


@sources.command(name="create")
@click.argument("uri")
@with_session
async def create_source(session: AsyncSession, uri: str) -> None:
    """Add a new code source."""
    repository = SourceRepository(session)
    service = SourceService(config.get_clone_dir(), repository)
    source = await service.create(uri)
    click.echo(f"Source created: {source.id}")


@cli.group()
def indexes() -> None:
    """Manage indexes."""


@indexes.command(name="create")
@click.argument("source_id")
@with_session
async def create_index(session: AsyncSession, source_id: int) -> None:
    """Create an index for a source."""
    source_repository = SourceRepository(session)
    source_service = SourceService(config.get_clone_dir(), source_repository)
    repository = IndexRepository(session)
    service = IndexService(repository, source_service)
    index = await service.create(source_id)
    click.echo(f"Index created: {index.id}")


@indexes.command(name="list")
@with_session
async def list_indexes(session: AsyncSession) -> None:
    """List all indexes."""
    source_repository = SourceRepository(session)
    source_service = SourceService(config.get_clone_dir(), source_repository)
    repository = IndexRepository(session)
    service = IndexService(repository, source_service)
    indexes = await service.list_indexes()

    # Define headers and data
    headers = [
        "ID",
        "Created At",
        "Updated At",
        "Num Snippets",
    ]
    data = [
        [
            index.id,
            index.created_at,
            index.updated_at,
            index.num_snippets,
        ]
        for index in indexes
    ]

    # Create and display the table
    table = Table(headers=headers, data=data)
    click.echo(table)


@indexes.command(name="run")
@click.argument("index_id")
@with_session
async def run_index(session: AsyncSession, index_id: int) -> None:
    """Run an index."""
    source_repository = SourceRepository(session)
    source_service = SourceService(config.get_clone_dir(), source_repository)
    repository = IndexRepository(session)
    service = IndexService(repository, source_service)
    await service.run(index_id)


@cli.command()
@click.argument("query")
@with_session
async def retrieve(session: AsyncSession, query: str) -> None:
    """Retrieve snippets from the database."""
    repository = RetrievalRepository(session)
    service = RetrievalService(repository)
    # Temporary request while we don't have all search capabilities
    snippets = await service.retrieve(RetrievalRequest(keywords=[query]))

    for snippet in snippets:
        click.echo(f"{snippet.uri}")
        click.echo(snippet.content)
        click.echo()


@cli.command()
@click.option("--host", default="127.0.0.1", help="Host to bind the server to")
@click.option("--port", default=8080, help="Port to bind the server to")
def serve(
    host: str,
    port: int,
) -> None:
    """Start the kodit server, which hosts the MCP server and the kodit API."""
    log = structlog.get_logger(__name__)
    log.info("Starting kodit server", host=host, port=port)
    log_event("kodit_server_started")
    os.environ["HELLO"] = "WORLD"

    # Configure uvicorn with graceful shutdown
    config = uvicorn.Config(
        "kodit.app:app",
        host=host,
        port=port,
        reload=False,
        log_config=None,  # Setting to None forces uvicorn to use our structlog setup
        access_log=False,  # Using own middleware for access logging
        timeout_graceful_shutdown=0,  # The mcp server does not support graceful shutdown
    )
    server = uvicorn.Server(config)

    def handle_sigint(signum: int, frame: Any) -> None:
        """Handle SIGINT (Ctrl+C)."""
        log.info("Received shutdown signal, force killing MCP connections")
        server.handle_exit(signum, frame)

    signal.signal(signal.SIGINT, handle_sigint)
    server.run()


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
