"""Command line interface for kodit."""

import signal
from pathlib import Path
from typing import Any

import click
import structlog
import uvicorn
from pytable_formatter import Cell, Table
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.application.factories.code_indexing_factory import (
    create_code_indexing_application_service,
)
from kodit.config import (
    AppContext,
    with_app_context,
    with_session,
)
from kodit.domain.errors import EmptySourceError
from kodit.domain.services.source_service import SourceService
from kodit.domain.value_objects import MultiSearchRequest, SnippetSearchFilters
from kodit.infrastructure.ui.progress import (
    create_lazy_progress_callback,
    create_multi_stage_progress_callback,
)
from kodit.log import configure_logging, configure_telemetry, log_event


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
@click.option(
    "--auto-index", is_flag=True, help="Index all configured auto-index sources"
)
@with_app_context
@with_session
async def index(
    session: AsyncSession,
    app_context: AppContext,
    sources: list[str],
    *,  # Force keyword-only arguments
    auto_index: bool,
) -> None:
    """List indexes, or index data sources."""
    log = structlog.get_logger(__name__)
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    service = create_code_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
    )

    if auto_index:
        log.info("Auto-indexing configuration", config=app_context.auto_indexing)
        auto_sources = app_context.auto_indexing.sources
        if not auto_sources:
            click.echo("No auto-index sources configured.")
            return

        click.echo(f"Auto-indexing {len(auto_sources)} configured sources...")
        sources = [source.uri for source in auto_sources]

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
        try:
            await service.run_index(index.id, indexing_progress_callback)
        except EmptySourceError as e:
            log.exception("Empty source error", error=e)
            msg = f"""{e}. This could mean:
• The repository contains no supported file types
• All files are excluded by ignore patterns
• The files contain no extractable code snippets
Please check the repository contents and try again.
"""
            click.echo(msg)


@cli.group()
def search() -> None:
    """Search for snippets in the database."""


# Utility for robust filter parsing
def _parse_filters(
    language: str | None,
    author: str | None,
    created_after: str | None,
    created_before: str | None,
    source_repo: str | None,
) -> SnippetSearchFilters | None:
    from datetime import datetime

    # Normalize language to lowercase if provided
    norm_language = language.lower() if language else None
    # Try to parse dates, raise error if invalid
    parsed_created_after = None
    if created_after:
        try:
            parsed_created_after = datetime.fromisoformat(created_after)
        except ValueError as err:
            raise ValueError(
                f"Invalid date format for --created-after: {created_after}. "
                "Expected ISO 8601 format (YYYY-MM-DD)"
            ) from err
    parsed_created_before = None
    if created_before:
        try:
            parsed_created_before = datetime.fromisoformat(created_before)
        except ValueError as err:
            raise ValueError(
                f"Invalid date format for --created-before: {created_before}. "
                "Expected ISO 8601 format (YYYY-MM-DD)"
            ) from err
    # Return None if no filters provided, otherwise return SnippetSearchFilters
    # Check if any original parameters were provided (not just the parsed values)
    if any(
        [
            language,
            author,
            created_after,
            created_before,
            source_repo,
        ]
    ):
        return SnippetSearchFilters(
            language=norm_language,
            author=author,
            created_after=parsed_created_after,
            created_before=parsed_created_before,
            source_repo=source_repo,
        )
    return None


@search.command()
@click.argument("query")
@click.option("--top-k", default=10, help="Number of snippets to retrieve")
@click.option(
    "--language", help="Filter by programming language (e.g., python, go, javascript)"
)
@click.option("--author", help="Filter by author name")
@click.option(
    "--created-after", help="Filter snippets created after this date (YYYY-MM-DD)"
)
@click.option(
    "--created-before", help="Filter snippets created before this date (YYYY-MM-DD)"
)
@click.option(
    "--source-repo", help="Filter by source repository (e.g., github.com/example/repo)"
)
@with_app_context
@with_session
async def code(  # noqa: PLR0913
    session: AsyncSession,
    app_context: AppContext,
    query: str,
    top_k: int,
    language: str | None,
    author: str | None,
    created_after: str | None,
    created_before: str | None,
    source_repo: str | None,
) -> None:
    """Search for snippets using semantic code search.

    This works best if your query is code.
    """
    log_event("kodit.cli.search.code")
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    service = create_code_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
    )

    filters = _parse_filters(
        language, author, created_after, created_before, source_repo
    )

    snippets = await service.search(
        MultiSearchRequest(code_query=query, top_k=top_k, filters=filters)
    )

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo(str(snippet))


@search.command()
@click.argument("keywords", nargs=-1)
@click.option("--top-k", default=10, help="Number of snippets to retrieve")
@click.option(
    "--language", help="Filter by programming language (e.g., python, go, javascript)"
)
@click.option("--author", help="Filter by author name")
@click.option(
    "--created-after", help="Filter snippets created after this date (YYYY-MM-DD)"
)
@click.option(
    "--created-before", help="Filter snippets created before this date (YYYY-MM-DD)"
)
@click.option(
    "--source-repo", help="Filter by source repository (e.g., github.com/example/repo)"
)
@with_app_context
@with_session
async def keyword(  # noqa: PLR0913
    session: AsyncSession,
    app_context: AppContext,
    keywords: list[str],
    top_k: int,
    language: str | None,
    author: str | None,
    created_after: str | None,
    created_before: str | None,
    source_repo: str | None,
) -> None:
    """Search for snippets using keyword search."""
    log_event("kodit.cli.search.keyword")
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    service = create_code_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
    )

    filters = _parse_filters(
        language, author, created_after, created_before, source_repo
    )

    snippets = await service.search(
        MultiSearchRequest(keywords=keywords, top_k=top_k, filters=filters)
    )

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo(str(snippet))


@search.command()
@click.argument("query")
@click.option("--top-k", default=10, help="Number of snippets to retrieve")
@click.option(
    "--language", help="Filter by programming language (e.g., python, go, javascript)"
)
@click.option("--author", help="Filter by author name")
@click.option(
    "--created-after", help="Filter snippets created after this date (YYYY-MM-DD)"
)
@click.option(
    "--created-before", help="Filter snippets created before this date (YYYY-MM-DD)"
)
@click.option(
    "--source-repo", help="Filter by source repository (e.g., github.com/example/repo)"
)
@with_app_context
@with_session
async def text(  # noqa: PLR0913
    session: AsyncSession,
    app_context: AppContext,
    query: str,
    top_k: int,
    language: str | None,
    author: str | None,
    created_after: str | None,
    created_before: str | None,
    source_repo: str | None,
) -> None:
    """Search for snippets using semantic text search.

    This works best if your query is text.
    """
    log_event("kodit.cli.search.text")
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    service = create_code_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
    )

    filters = _parse_filters(
        language, author, created_after, created_before, source_repo
    )

    snippets = await service.search(
        MultiSearchRequest(text_query=query, top_k=top_k, filters=filters)
    )

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo(str(snippet))


@search.command()
@click.option("--top-k", default=10, help="Number of snippets to retrieve")
@click.option("--keywords", required=True, help="Comma separated list of keywords")
@click.option("--code", required=True, help="Semantic code search query")
@click.option("--text", required=True, help="Semantic text search query")
@click.option(
    "--language", help="Filter by programming language (e.g., python, go, javascript)"
)
@click.option("--author", help="Filter by author name")
@click.option(
    "--created-after", help="Filter snippets created after this date (YYYY-MM-DD)"
)
@click.option(
    "--created-before", help="Filter snippets created before this date (YYYY-MM-DD)"
)
@click.option(
    "--source-repo", help="Filter by source repository (e.g., github.com/example/repo)"
)
@with_app_context
@with_session
async def hybrid(  # noqa: PLR0913
    session: AsyncSession,
    app_context: AppContext,
    top_k: int,
    keywords: str,
    code: str,
    text: str,
    language: str | None,
    author: str | None,
    created_after: str | None,
    created_before: str | None,
    source_repo: str | None,
) -> None:
    """Search for snippets using hybrid search."""
    log_event("kodit.cli.search.hybrid")
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    service = create_code_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
    )

    # Parse keywords into a list of strings
    keywords_list = [k.strip().lower() for k in keywords.split(",")]

    filters = _parse_filters(
        language, author, created_after, created_before, source_repo
    )

    snippets = await service.search(
        MultiSearchRequest(
            keywords=keywords_list,
            code_query=code,
            text_query=text,
            top_k=top_k,
            filters=filters,
        )
    )

    if len(snippets) == 0:
        click.echo("No snippets found")
        return

    for snippet in snippets:
        click.echo(str(snippet))


@cli.group()
def show() -> None:
    """Show information about elements in the database."""


@show.command()
@click.option("--by-path", help="File or directory path to search for snippets")
@click.option("--by-source", help="Source URI to filter snippets by")
@with_app_context
@with_session
async def snippets(
    session: AsyncSession,
    app_context: AppContext,
    by_path: str | None,
    by_source: str | None,
) -> None:
    """Show snippets with optional filtering by path or source."""
    log_event("kodit.cli.show.snippets")
    source_service = SourceService(
        clone_dir=app_context.get_clone_dir(),
        session_factory=lambda: session,
    )
    service = create_code_indexing_application_service(
        app_context=app_context,
        session=session,
        source_service=source_service,
    )
    snippets = await service.list_snippets(file_path=by_path, source_uri=by_source)
    for snippet in snippets:
        click.echo(str(snippet))


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
        return

    print(f"kodit {_version.__version__}")  # noqa: T201


if __name__ == "__main__":
    cli()
