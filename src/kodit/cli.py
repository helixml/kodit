# ruff: noqa: ARG001
"""Command line interface for kodit."""

import signal
import warnings
from pathlib import Path
from typing import Any

import click
import structlog
import uvicorn

from kodit.config import (
    AppContext,
    with_app_context,
    wrap_async,
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


@cli.command()
@click.argument("sources", nargs=-1)
@click.option("--sync", is_flag=True, help="Sync existing indexes with their remotes")
@with_app_context
@wrap_async
async def index(
    app_context: AppContext,
    sources: list[str],
    *,  # Force keyword-only arguments
    sync: bool,
) -> None:
    """List indexes, index data sources, or sync existing indexes."""
    click.echo("Not implemented")


@cli.group()
def search() -> None:
    """Search for snippets in the database."""


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
    click.echo("Not implemented")


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
    click.echo("Not implemented")


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
    click.echo("Not implemented")


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
    click.echo("Not implemented")


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
    click.echo("Not implemented")


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
