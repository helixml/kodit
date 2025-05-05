"""Command line interface for kodit."""

import os

import click
import structlog
import uvicorn
from dotenv import dotenv_values

from kodit.logging import LogFormat, configure_logging, disable_posthog, log_event

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


if __name__ == "__main__":
    cli()
