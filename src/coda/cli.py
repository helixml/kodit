"""Command line interface for Coda."""

import os

import click
import structlog
import uvicorn
from dotenv import dotenv_values

from coda.logging import setup_logging

env_vars = dict(dotenv_values())
os.environ.update(env_vars)


@click.group(context_settings={"auto_envvar_prefix": "CODA", "show_default": True})
@click.option("--log-json", is_flag=True, help="Enable JSON logging")
@click.option("--log-level", default="INFO", help="Log level")
def cli(*, log_json: bool, log_level: str) -> None:
    """Coda CLI - Code indexing for better AI code generation."""
    setup_logging(json_logs=log_json, log_level=log_level)


@cli.command()
@click.option("--host", default="127.0.0.1", help="Host to bind the server to")
@click.option("--port", default=8000, help="Port to bind the server to")
@click.option("--reload", is_flag=True, help="Enable auto-reload for development")
def serve(*, host: str, port: int, reload: bool) -> None:
    """Start the Coda server, which hosts the MCP server and the Coda API."""
    log = structlog.get_logger(__name__)
    log.info("Starting Coda server", host=host, port=port, reload=reload)
    uvicorn.run(
        "coda.app:app",
        host=host,
        port=port,
        reload=reload,
        log_config=None,  # Setting to None forces uvicorn to use our structlog setup
        access_log=False,  # Using own middleware for access logging
    )


if __name__ == "__main__":
    cli()
