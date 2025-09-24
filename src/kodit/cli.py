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
    warnings.filterwarnings("ignore", category=DeprecationWarning, module="httpx")

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
