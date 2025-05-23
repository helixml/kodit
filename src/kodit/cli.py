"""Command line interface for kodit."""

import os
import signal
from pathlib import Path
from typing import Any

import click
import structlog
import uvicorn
from pytable_formatter import Cell, Table
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import (
    DEFAULT_BASE_DIR,
    DEFAULT_DB_URL,
    DEFAULT_DISABLE_TELEMETRY,
    DEFAULT_EMBEDDING_MODEL_NAME,
    DEFAULT_LOG_FORMAT,
    DEFAULT_LOG_LEVEL,
    AppContext,
    with_app_context,
    with_session,
)
from kodit.indexing.repository import IndexRepository
from kodit.indexing.service import IndexService
from kodit.log import configure_logging, configure_telemetry, log_event
from kodit.retreival.repository import RetrievalRepository
from kodit.retreival.service import RetrievalRequest, RetrievalService
from kodit.sources.repository import SourceRepository
from kodit.sources.service import SourceService


@click.group(context_settings={"max_content_width": 100})
@click.option("--log-level", help=f"Log level [default: {DEFAULT_LOG_LEVEL}]")
@click.option("--log-format", help=f"Log format [default: {DEFAULT_LOG_FORMAT}]")
@click.option(
    "--disable-telemetry",
    is_flag=True,
    help=f"Disable telemetry [default: {DEFAULT_DISABLE_TELEMETRY}]",
)
@click.option("--db-url", help=f"Database URL [default: {DEFAULT_DB_URL}]")
@click.option("--data-dir", help=f"Data directory [default: {DEFAULT_BASE_DIR}]")
@click.option(
    "--env-file",
    help="Path to a .env file [default: .env]",
    type=click.Path(
        exists=True,
        dir_okay=False,
        resolve_path=True,
        path_type=Path,
    ),
)
@click.pass_context
def cli(  # noqa: PLR0913
    ctx: click.Context,
    log_level: str | None,
    log_format: str | None,
    disable_telemetry: bool | None,
    db_url: str | None,
    data_dir: str | None,
    env_file: Path | None,
) -> None:
    """kodit CLI - Code indexing for better AI code generation."""  # noqa: D403
    config = AppContext()
    # First check if env-file is set and reload config if it is
    if env_file:
        config = AppContext(_env_file=env_file)  # type: ignore[reportCallIssue]

    # Now override with CLI arguments, if set
    if data_dir:
        config.data_dir = Path(data_dir)
    if db_url:
        config.db_url = db_url
    if log_level:
        config.log_level = log_level
    if log_format:
        config.log_format = log_format
    if disable_telemetry:
        config.disable_telemetry = disable_telemetry
    configure_logging(config)
    configure_telemetry(config)

    # Set the app context in the click context for downstream cli
    ctx.obj = config


@cli.command()
@click.argument("sources", nargs=-1)
@with_app_context
@with_session
async def index(
    session: AsyncSession,
    app_context: AppContext,
    sources: list[str],
) -> None:
    """List indexes, or index data sources."""
    source_repository = SourceRepository(session)
    source_service = SourceService(app_context.get_clone_dir(), source_repository)
    repository = IndexRepository(session)
    service = IndexService(
        repository,
        source_service,
        app_context.get_data_dir(),
        embedding_model_name=DEFAULT_EMBEDDING_MODEL_NAME,
    )

    if not sources:
        # No source specified, list all indexes
        indexes = await service.list_indexes()
        headers: list[str | Cell] = [
            "ID",
            "Created At",
            "Updated At",
            "Source",
            "Num Snippets",
        ]
        data = [
            [
                index.id,
                index.created_at,
                index.updated_at,
                index.source,
                index.num_snippets,
            ]
            for index in indexes
        ]
        click.echo(Table(headers=headers, data=data))
        return
    # Handle source indexing
    for source in sources:
        if Path(source).is_file():
            msg = "File indexing is not implemented yet"
            raise click.UsageError(msg)

        # Index directory
        s = await source_service.create(source)
        index = await service.create(s.id)
        await service.run(index.id)


@cli.group()
def search() -> None:
    """Search for snippets in the database."""


@search.command()
@click.argument("query")
@click.option("--top-k", default=10, help="Number of snippets to retrieve")
@with_app_context
@with_session
async def code(
    session: AsyncSession,
    app_context: AppContext,
    query: str,
    top_k: int,
) -> None:
    """Search for snippets using semantic code search.

    This works best if your query is code.
    """
    repository = RetrievalRepository(session)
    service = RetrievalService(
        repository,
        app_context.get_data_dir(),
        embedding_model_name=DEFAULT_EMBEDDING_MODEL_NAME,
    )

    snippets = await service.retrieve(RetrievalRequest(code_query=query, top_k=top_k))

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo("-" * 80)
        click.echo(f"{snippet.uri}")
        click.echo(snippet.content)
        click.echo("-" * 80)
        click.echo()


@search.command()
@click.argument("keywords", nargs=-1)
@click.option("--top-k", default=10, help="Number of snippets to retrieve")
@with_app_context
@with_session
async def keyword(
    session: AsyncSession,
    app_context: AppContext,
    keywords: list[str],
    top_k: int,
) -> None:
    """Search for snippets using keyword search."""
    repository = RetrievalRepository(session)
    service = RetrievalService(
        repository,
        app_context.get_data_dir(),
        embedding_model_name=DEFAULT_EMBEDDING_MODEL_NAME,
    )

    snippets = await service.retrieve(RetrievalRequest(keywords=keywords, top_k=top_k))

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo("-" * 80)
        click.echo(f"{snippet.uri}")
        click.echo(snippet.content)
        click.echo("-" * 80)
        click.echo()


@search.command()
@click.option("--top-k", default=10, help="Number of snippets to retrieve")
@click.option("--keywords", required=True, help="Comma separated list of keywords")
@click.option("--code", required=True, help="Semantic code search query")
@with_app_context
@with_session
async def hybrid(
    session: AsyncSession,
    app_context: AppContext,
    top_k: int,
    keywords: str,
    code: str,
) -> None:
    """Search for snippets using hybrid search."""
    repository = RetrievalRepository(session)
    service = RetrievalService(
        repository,
        app_context.get_data_dir(),
        embedding_model_name=DEFAULT_EMBEDDING_MODEL_NAME,
    )

    # Parse keywords into a list of strings
    keywords_list = [k.strip().lower() for k in keywords.split(",")]

    snippets = await service.retrieve(
        RetrievalRequest(keywords=keywords_list, code_query=code, top_k=top_k)
    )

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo("-" * 80)
        click.echo(f"{snippet.uri}")
        click.echo(snippet.content)
        click.echo("-" * 80)
        click.echo()


@cli.command()
@click.option("--host", default="127.0.0.1", help="Host to bind the server to")
@click.option("--port", default=8080, help="Port to bind the server to")
@with_app_context
def serve(
    app_context: AppContext,
    host: str,
    port: int,
) -> None:
    """Start the kodit server, which hosts the MCP server and the kodit API."""
    log = structlog.get_logger(__name__)
    log.info("Starting kodit server", host=host, port=port)
    log_event("kodit_server_started")

    # Dump AppContext to a dictionary of strings, and set the env vars
    app_context_dict = {k: str(v) for k, v in app_context.model_dump().items()}
    os.environ.update(app_context_dict)

    # Configure uvicorn with graceful shutdown
    config = uvicorn.Config(
        "kodit.app:app",
        host=host,
        port=port,
        reload=False,
        log_config=None,  # Setting to None forces uvicorn to use our structlog setup
        access_log=False,  # Using own middleware for access logging
        timeout_graceful_shutdown=0,  # The mcp server does not shutdown cleanly, force
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
