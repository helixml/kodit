"""Command line interface for kodit."""

import signal
import warnings
from pathlib import Path
from typing import Any

import click
import structlog
import uvicorn
from pytable_formatter import Cell, Table  # type: ignore[import-untyped]

from kodit.application.factories.code_indexing_factory import (
    create_code_indexing_application_service,
)
from kodit.config import (
    AppContext,
    with_app_context,
    wrap_async,
)
from kodit.domain.errors import EmptySourceError
from kodit.domain.services.index_query_service import IndexQueryService
from kodit.domain.value_objects import (
    MultiSearchRequest,
    MultiSearchResult,
    SnippetSearchFilters,
)
from kodit.infrastructure.api.client import IndexClient, SearchClient
from kodit.infrastructure.indexing.fusion_service import ReciprocalRankFusionService
from kodit.infrastructure.sqlalchemy.index_repository import SqlAlchemyIndexRepository
from kodit.infrastructure.ui.progress import (
    create_lazy_progress_callback,
    create_multi_stage_progress_callback,
)
from kodit.log import configure_logging, configure_telemetry, log_event
from kodit.mcp import create_stdio_mcp_server


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
        config = AppContext(_env_file=env_file)  # type: ignore[call-arg]

    configure_logging(config)
    configure_telemetry(config)

    # Set the app context in the click context for downstream cli
    ctx.obj = config


async def _handle_auto_index(
    app_context: AppContext,
    sources: list[str],  # noqa: ARG001
) -> list[str]:
    """Handle auto-index option and return sources to process."""
    log = structlog.get_logger(__name__)
    log.info("Auto-indexing configuration", config=app_context.auto_indexing)
    if not app_context.auto_indexing or not app_context.auto_indexing.sources:
        click.echo("No auto-index sources configured.")
        return []
    auto_sources = app_context.auto_indexing.sources
    click.echo(f"Auto-indexing {len(auto_sources)} configured sources...")
    return [source.uri for source in auto_sources]


async def _handle_sync(
    service: Any,
    index_query_service: IndexQueryService,
    sources: list[str],
) -> None:
    """Handle sync operation."""
    log = structlog.get_logger(__name__)
    log_event("kodit.cli.index.sync")

    # Get all existing indexes
    all_indexes = await index_query_service.list_indexes()

    if not all_indexes:
        click.echo("No existing indexes found to sync.")
        return

    # Filter indexes if specific sources are provided
    indexes_to_sync = all_indexes
    if sources:
        # Filter indexes that match the provided sources
        source_uris = set(sources)
        indexes_to_sync = [
            index
            for index in all_indexes
            if str(index.source.working_copy.remote_uri) in source_uris
        ]

        if not indexes_to_sync:
            click.echo(
                f"No indexes found for the specified sources: {', '.join(sources)}"
            )
            return

    click.echo(f"Syncing {len(indexes_to_sync)} indexes...")

    # Sync each index
    for index in indexes_to_sync:
        click.echo(f"Syncing: {index.source.working_copy.remote_uri}")

        # Create progress callback for this sync operation
        progress_callback = create_multi_stage_progress_callback()

        try:
            await service.run_index(index, progress_callback)
            click.echo(f"✓ Sync completed: {index.source.working_copy.remote_uri}")
        except Exception as e:
            log.exception("Sync failed", index_id=index.id, error=e)
            click.echo(f"✗ Sync failed: {index.source.working_copy.remote_uri} - {e}")


async def _handle_list_indexes(index_query_service: IndexQueryService) -> None:
    """Handle listing all indexes."""
    log_event("kodit.cli.index.list")
    # No source specified, list all indexes
    indexes = await index_query_service.list_indexes()
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
            index.source.working_copy.remote_uri,
            len(index.source.working_copy.files),
        ]
        for index in indexes
    ]
    click.echo(Table(headers=headers, data=data))


@cli.command()
@click.argument("sources", nargs=-1)
@click.option(
    "--auto-index", is_flag=True, help="Index all configured auto-index sources"
)
@click.option("--sync", is_flag=True, help="Sync existing indexes with their remotes")
@with_app_context
@wrap_async
async def index(
    app_context: AppContext,
    sources: list[str],
    *,  # Force keyword-only arguments
    auto_index: bool,
    sync: bool,
) -> None:
    """List indexes, index data sources, or sync existing indexes."""
    if not app_context.is_remote:
        # Local mode - use existing implementation
        await _index_local(app_context, sources, auto_index, sync)
    else:
        # Remote mode - use API clients
        await _index_remote(app_context, sources, auto_index, sync)


async def _index_local(
    app_context: AppContext,
    sources: list[str],
    auto_index: bool,  # noqa: FBT001
    sync: bool,  # noqa: FBT001
) -> None:
    """Handle index operations in local mode."""
    log = structlog.get_logger(__name__)

    # Get database session
    db = await app_context.get_db()
    async with db.session_factory() as session:
        service = create_code_indexing_application_service(
            app_context=app_context,
            session=session,
        )
        index_query_service = IndexQueryService(
            index_repository=SqlAlchemyIndexRepository(session=session),
            fusion_service=ReciprocalRankFusionService(),
        )

        if auto_index:
            sources = await _handle_auto_index(app_context, sources)
            if not sources:
                return

        if sync:
            await _handle_sync(service, index_query_service, sources)
            return

        if not sources:
            await _handle_list_indexes(index_query_service)
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
            index = await service.create_index_from_uri(source, progress_callback)

            # Create a new progress callback for the indexing operations
            indexing_progress_callback = create_multi_stage_progress_callback()
            try:
                await service.run_index(index, indexing_progress_callback)
            except EmptySourceError as e:
                log.exception("Empty source error", error=e)
                msg = f"""{e}. This could mean:
• The repository contains no supported file types
• All files are excluded by ignore patterns
• The files contain no extractable code snippets
Please check the repository contents and try again.
"""
                click.echo(msg)


async def _index_remote(
    app_context: AppContext,
    sources: list[str],
    auto_index: bool,  # noqa: FBT001
    sync: bool,  # noqa: FBT001
) -> None:
    """Handle index operations in remote mode."""
    if sync:
        # Sync operation not available in remote mode
        click.echo("⚠️  Warning: Index sync is not implemented in remote mode")
        click.echo("Please use the server's auto-sync functionality or sync locally")
        return

    if auto_index:
        click.echo("⚠️  Warning: Auto-index is not implemented in remote mode")
        click.echo("Please configure sources to be auto-indexed on the server")
        return

    # Create API client
    index_client = IndexClient(
        base_url=app_context.remote.server_url or "",
        api_key=app_context.remote.api_key,
        timeout=app_context.remote.timeout,
        max_retries=app_context.remote.max_retries,
        verify_ssl=app_context.remote.verify_ssl,
    )

    try:
        if not sources:
            # List indexes
            indexes = await index_client.list_indexes()
            _display_indexes_remote(indexes)
            return

        # Create new indexes
        for source in sources:
            click.echo(f"Creating index for: {source}")
            index = await index_client.create_index(source)
            click.echo(f"✓ Index created with ID: {index.id}")

    finally:
        await index_client.close()


def _display_indexes_remote(indexes: list[Any]) -> None:
    """Display indexes for remote mode."""
    headers = [
        "ID",
        "Created At",
        "Updated At",
        "Source",
    ]
    data = [
        [
            index.id,
            index.attributes.created_at,
            index.attributes.updated_at,
            index.attributes.uri,
        ]
        for index in indexes
    ]

    click.echo(Table(headers=headers, data=data))


async def _search_local(  # noqa: PLR0913
    app_context: AppContext,
    keywords: list[str] | None = None,
    code_query: str | None = None,
    text_query: str | None = None,
    top_k: int = 10,
    language: str | None = None,
    author: str | None = None,
    created_after: str | None = None,
    created_before: str | None = None,
    source_repo: str | None = None,
    output_format: str = "text",
    event_name: str = "kodit.cli.search",
) -> None:
    """Handle search operations in local mode."""
    log_event(event_name)

    # Get database session
    db = await app_context.get_db()
    async with db.session_factory() as session:
        service = create_code_indexing_application_service(
            app_context=app_context,
            session=session,
        )

        filters = _parse_filters(
            language, author, created_after, created_before, source_repo
        )

        snippets = await service.search(
            MultiSearchRequest(
                keywords=keywords,
                code_query=code_query,
                text_query=text_query,
                top_k=top_k,
                filters=filters,
            )
        )

        if len(snippets) == 0:
            click.echo("No snippets found")
            return

        if output_format == "text":
            click.echo(MultiSearchResult.to_string(snippets))
        elif output_format == "json":
            click.echo(MultiSearchResult.to_jsonlines(snippets))


async def _search_remote(  # noqa: PLR0913
    app_context: AppContext,
    keywords: list[str] | None = None,
    code_query: str | None = None,
    text_query: str | None = None,
    top_k: int = 10,
    language: str | None = None,
    author: str | None = None,
    created_after: str | None = None,
    created_before: str | None = None,
    source_repo: str | None = None,
    output_format: str = "text",
) -> None:
    """Handle search operations in remote mode."""
    from datetime import datetime

    # Parse date filters for API
    parsed_start_date = None
    if created_after:
        try:
            parsed_start_date = datetime.fromisoformat(created_after)
        except ValueError as err:
            raise ValueError(
                f"Invalid date format for --created-after: {created_after}. "
                "Expected ISO 8601 format (YYYY-MM-DD)"
            ) from err

    parsed_end_date = None
    if created_before:
        try:
            parsed_end_date = datetime.fromisoformat(created_before)
        except ValueError as err:
            raise ValueError(
                f"Invalid date format for --created-before: {created_before}. "
                "Expected ISO 8601 format (YYYY-MM-DD)"
            ) from err

    # Create API client
    search_client = SearchClient(
        base_url=app_context.remote.server_url or "",
        api_key=app_context.remote.api_key,
        timeout=app_context.remote.timeout,
        max_retries=app_context.remote.max_retries,
        verify_ssl=app_context.remote.verify_ssl,
    )

    try:
        # Perform search
        snippets = await search_client.search(
            keywords=keywords,
            code_query=code_query,
            text_query=text_query,
            limit=top_k,
            languages=[language] if language else None,
            authors=[author] if author else None,
            start_date=parsed_start_date,
            end_date=parsed_end_date,
            sources=[source_repo] if source_repo else None,
        )

        if len(snippets) == 0:
            click.echo("No snippets found")
            return

        if output_format == "text":
            _display_snippets_remote_text(snippets)
        elif output_format == "json":
            _display_snippets_remote_json(snippets)

    finally:
        await search_client.close()


def _display_snippets_remote_text(snippets: list[Any]) -> None:
    """Display snippets in text format for remote mode."""
    for i, snippet in enumerate(snippets):
        click.echo(f"\n--- Snippet {i + 1} ---")
        click.echo(f"ID: {snippet.id}")
        click.echo(f"Language: {snippet.attributes.language}")
        click.echo(f"Source: {snippet.attributes.source_uri}")
        click.echo(f"Path: {snippet.attributes.relative_path}")
        click.echo(f"Authors: {', '.join(snippet.attributes.authors)}")
        click.echo(f"Summary: {snippet.attributes.summary}")
        click.echo("\nContent:")
        click.echo(snippet.attributes.content)


def _display_snippets_remote_json(snippets: list[Any]) -> None:
    """Display snippets in JSON format for remote mode."""
    import json

    for snippet in snippets:
        snippet_data = {
            "id": snippet.id,
            "language": snippet.attributes.language,
            "source_uri": snippet.attributes.source_uri,
            "relative_path": snippet.attributes.relative_path,
            "authors": snippet.attributes.authors,
            "summary": snippet.attributes.summary,
            "content": snippet.attributes.content,
            "created_at": snippet.attributes.created_at.isoformat(),
            "updated_at": snippet.attributes.updated_at.isoformat(),
        }
        click.echo(json.dumps(snippet_data))


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
@click.option("--output-format", default="text", help="Format to display snippets in")
@with_app_context
@wrap_async
async def code(  # noqa: PLR0913
    app_context: AppContext,
    query: str,
    top_k: int,
    language: str | None,
    author: str | None,
    created_after: str | None,
    created_before: str | None,
    source_repo: str | None,
    output_format: str,
) -> None:
    """Search for snippets using semantic code search.

    This works best if your query is code.
    """
    if not app_context.is_remote:
        await _search_local(
            app_context,
            code_query=query,
            top_k=top_k,
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
            output_format=output_format,
            event_name="kodit.cli.search.code",
        )
    else:
        await _search_remote(
            app_context,
            code_query=query,
            top_k=top_k,
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
            output_format=output_format,
        )


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
@click.option("--output-format", default="text", help="Format to display snippets in")
@with_app_context
@wrap_async
async def keyword(  # noqa: PLR0913
    app_context: AppContext,
    keywords: list[str],
    top_k: int,
    language: str | None,
    author: str | None,
    created_after: str | None,
    created_before: str | None,
    source_repo: str | None,
    output_format: str,
) -> None:
    """Search for snippets using keyword search."""
    # Override remote configuration if provided via CLI
    if not app_context.is_remote:
        await _search_local(
            app_context,
            keywords=keywords,
            top_k=top_k,
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
            output_format=output_format,
            event_name="kodit.cli.search.keyword",
        )
    else:
        await _search_remote(
            app_context,
            keywords=keywords,
            top_k=top_k,
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
            output_format=output_format,
        )


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
@click.option("--output-format", default="text", help="Format to display snippets in")
@with_app_context
@wrap_async
async def text(  # noqa: PLR0913
    app_context: AppContext,
    query: str,
    top_k: int,
    language: str | None,
    author: str | None,
    created_after: str | None,
    created_before: str | None,
    source_repo: str | None,
    output_format: str,
) -> None:
    """Search for snippets using semantic text search.

    This works best if your query is text.
    """
    if not app_context.is_remote:
        await _search_local(
            app_context,
            text_query=query,
            top_k=top_k,
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
            output_format=output_format,
            event_name="kodit.cli.search.text",
        )
    else:
        await _search_remote(
            app_context,
            text_query=query,
            top_k=top_k,
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
            output_format=output_format,
        )


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
@click.option("--output-format", default="text", help="Format to display snippets in")
@with_app_context
@wrap_async
async def hybrid(  # noqa: PLR0913
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
    output_format: str,
) -> None:
    """Search for snippets using hybrid search."""
    # Parse keywords into a list of strings
    keywords_list = [k.strip().lower() for k in keywords.split(",")]

    if not app_context.is_remote:
        await _search_local(
            app_context,
            keywords=keywords_list,
            code_query=code,
            text_query=text,
            top_k=top_k,
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
            output_format=output_format,
            event_name="kodit.cli.search.hybrid",
        )
    else:
        await _search_remote(
            app_context,
            keywords=keywords_list,
            code_query=code,
            text_query=text,
            top_k=top_k,
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
            output_format=output_format,
        )


@cli.group()
def show() -> None:
    """Show information about elements in the database."""


@show.command()
@click.option("--by-path", help="File or directory path to search for snippets")
@click.option("--by-source", help="Source URI to filter snippets by")
@click.option("--output-format", default="text", help="Format to display snippets in")
@with_app_context
@wrap_async
async def snippets(
    app_context: AppContext,
    by_path: str | None,
    by_source: str | None,
    output_format: str,
) -> None:
    """Show snippets with optional filtering by path or source."""
    if not app_context.is_remote:
        # Local mode
        log_event("kodit.cli.show.snippets")
        db = await app_context.get_db()
        async with db.session_factory() as session:
            service = create_code_indexing_application_service(
                app_context=app_context,
                session=session,
            )
            snippets = await service.list_snippets(
                file_path=by_path, source_uri=by_source
            )
            if output_format == "text":
                click.echo(MultiSearchResult.to_string(snippets))
            elif output_format == "json":
                click.echo(MultiSearchResult.to_jsonlines(snippets))
    else:
        # Remote mode - not supported
        click.echo("⚠️  Warning: 'show snippets' is not implemented in remote mode")
        click.echo(
            "This functionality is only available when connected directly "
            "to the database"
        )
        click.echo("Use 'kodit search' commands instead for remote snippet retrieval")


@cli.command()
@click.option("--host", default="127.0.0.1", help="Host to bind the server to")
@click.option("--port", default=8080, help="Port to bind the server to")
def serve(
    host: str,
    port: int,
) -> None:
    """Start the kodit HTTP/SSE server with FastAPI integration."""
    log = structlog.get_logger(__name__)
    log.info("Starting kodit server", host=host, port=port)
    log_event("kodit.cli.serve")

    # Disable uvicorn's websockets deprecation warnings
    warnings.filterwarnings("ignore", category=DeprecationWarning, module="websockets")
    warnings.filterwarnings("ignore", category=DeprecationWarning, module="uvicorn")

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
def stdio() -> None:
    """Start the kodit MCP server in STDIO mode."""
    log_event("kodit.cli.stdio")
    create_stdio_mcp_server()


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
