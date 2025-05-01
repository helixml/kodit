import os

import click
import structlog
import uvicorn
from pydantic import parse_obj_as

from coda.logging import setup_logging, uvicorn_log_config

LOG_JSON_FORMAT = parse_obj_as(bool, os.getenv("LOG_JSON_FORMAT", False))
LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
setup_logging(json_logs=LOG_JSON_FORMAT, log_level=LOG_LEVEL)

log = structlog.get_logger(__name__)


@click.group()
def cli():
    """Coda CLI - Code indexing for better AI code generation"""
    pass


@cli.command()
@click.option("--host", default="127.0.0.1", help="Host to bind the server to")
@click.option("--port", default=8000, help="Port to bind the server to")
@click.option("--reload", is_flag=True, help="Enable auto-reload for development")
def serve(host: str, port: int, reload: bool):
    """Start the Coda server, which hosts the MCP server and the Coda API."""
    log.info("Starting Coda server", host=host, port=port, reload=reload)
    uvicorn.run(
        "coda.app:app",
        host=host,
        port=port,
        reload=reload,
        log_config=uvicorn_log_config,
    )


if __name__ == "__main__":
    cli()
