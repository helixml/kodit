"""Command line interface for kodit."""

import asyncio
import signal
from pathlib import Path
from typing import Any

import click
import structlog
import uvicorn
from pytable_formatter import Cell, Table
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.services.snippet_application_service import (
    SnippetApplicationService,
)
from kodit.config import (
    AppContext,
    with_app_context,
    with_session,
)
from kodit.domain.services.source_service import SourceService
from kodit.domain.value_objects import MultiSearchRequest
from kodit.infrastructure.indexing.indexing_factory import (
    create_indexing_application_service,
)
from kodit.infrastructure.snippet_extraction.snippet_extraction_factory import (
    create_snippet_extraction_domain_service,
    create_snippet_repositories,
)
from kodit.infrastructure.ui.progress import (
    create_lazy_progress_callback,
    create_multi_stage_progress_callback,
)
from kodit.log import configure_logging, configure_telemetry, log_event


def create_snippet_application_service(
    session: AsyncSession,
) -> SnippetApplicationService:
    """Create a snippet application service with all dependencies.

    Args:
        session: SQLAlchemy session

    Returns:
        Configured snippet application service

    """
    # Create domain service
    snippet_extraction_service = create_snippet_extraction_domain_service()

    # Create repositories
    snippet_repository, file_repository = create_snippet_repositories(session)

    # Create application service
    return SnippetApplicationService(
        snippet_extraction_service=snippet_extraction_service,
        snippet_repository=snippet_repository,
        file_repository=file_repository,
    )


@click.group(context_settings={"max_content_width": 100})
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
def cli(
    ctx: click.Context,
    env_file: Path | None,
) -> None:
    """kodit CLI - Code indexing for better AI code generation."""  # noqa: D403
    config = AppContext()
    # First check if env-file is set and reload config if it is
    if env_file:
        config = AppContext(_env_file=env_file)  # type: ignore[reportCallIssue]

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
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    snippet_service = create_snippet_application_service(session)
    service = create_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
        snippet_application_service=snippet_service,
    )

    if not sources:
        log_event("kodit.cli.index.list")
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

        # Index source with progress
        log_event("kodit.cli.index.create")

        # Create a lazy progress callback that only shows progress when needed
        progress_callback = create_lazy_progress_callback()
        s = await source_service.create(source, progress_callback)

        index = await service.create_index(s.id)

        # Create a new progress callback for the indexing operations
        indexing_progress_callback = create_multi_stage_progress_callback()
        await service.run_index(index.id, indexing_progress_callback)


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
    log_event("kodit.cli.search.code")
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    snippet_service = create_snippet_application_service(session)
    service = create_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
        snippet_application_service=snippet_service,
    )

    snippets = await service.search(MultiSearchRequest(code_query=query, top_k=top_k))

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo("-" * 80)
        click.echo(f"{snippet.uri}")
        click.echo(f"Original scores: {snippet.original_scores}")
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
    log_event("kodit.cli.search.keyword")
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    snippet_service = create_snippet_application_service(session)
    service = create_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
        snippet_application_service=snippet_service,
    )

    snippets = await service.search(MultiSearchRequest(keywords=keywords, top_k=top_k))

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo("-" * 80)
        click.echo(f"{snippet.uri}")
        click.echo(f"Original scores: {snippet.original_scores}")
        click.echo(snippet.content)
        click.echo("-" * 80)
        click.echo()


@search.command()
@click.argument("query")
@click.option("--top-k", default=10, help="Number of snippets to retrieve")
@with_app_context
@with_session
async def text(
    session: AsyncSession,
    app_context: AppContext,
    query: str,
    top_k: int,
) -> None:
    """Search for snippets using semantic text search.

    This works best if your query is text.
    """
    log_event("kodit.cli.search.text")
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    snippet_service = create_snippet_application_service(session)
    service = create_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
        snippet_application_service=snippet_service,
    )

    snippets = await service.search(MultiSearchRequest(text_query=query, top_k=top_k))

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo("-" * 80)
        click.echo(f"{snippet.uri}")
        click.echo(f"Original scores: {snippet.original_scores}")
        click.echo(snippet.content)
        click.echo("-" * 80)
        click.echo()


@search.command()
@click.option("--top-k", default=10, help="Number of snippets to retrieve")
@click.option("--keywords", required=True, help="Comma separated list of keywords")
@click.option("--code", required=True, help="Semantic code search query")
@click.option("--text", required=True, help="Semantic text search query")
@with_app_context
@with_session
async def hybrid(  # noqa: PLR0913
    session: AsyncSession,
    app_context: AppContext,
    top_k: int,
    keywords: str,
    code: str,
    text: str,
) -> None:
    """Search for snippets using hybrid search."""
    log_event("kodit.cli.search.hybrid")
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    snippet_service = create_snippet_application_service(session)
    service = create_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
        snippet_application_service=snippet_service,
    )

    # Parse keywords into a list of strings
    keywords_list = [k.strip().lower() for k in keywords.split(",")]

    snippets = await service.search(
        MultiSearchRequest(
            keywords=keywords_list,
            code_query=code,
            text_query=text,
            top_k=top_k,
        )
    )

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo("-" * 80)
        click.echo(f"{snippet.uri}")
        click.echo(f"Original scores: {snippet.original_scores}")
        click.echo(snippet.content)
        click.echo("-" * 80)
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
    log_event("kodit.cli.serve")

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
    asyncio.run(cli())
