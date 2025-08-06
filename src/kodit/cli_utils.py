"""Utilities for CLI commands with remote/local mode support."""

from collections.abc import Callable
from functools import wraps
from typing import TYPE_CHECKING, Any

import click

from kodit.infrastructure.api.client import IndexClient, SearchClient

if TYPE_CHECKING:
    from kodit.config import AppContext


def with_client(f: Callable) -> Callable:
    """Provide appropriate client based on configuration.

    This decorator automatically detects whether to run in local or remote mode
    based on the presence of remote.server_url in the configuration. In remote
    mode, it provides API clients. In local mode, it behaves like the existing
    with_session decorator.
    """

    @wraps(f)
    async def wrapper(*args: Any, **kwargs: Any) -> Any:
        ctx = click.get_current_context()
        app_context: AppContext = ctx.obj

        # Auto-detect mode based on remote.server_url presence
        if not app_context.is_remote:
            # Local mode - use existing database session approach
            from kodit.config import with_session

            # Apply the session decorator to the original function
            session_wrapped = with_session(f)
            # Remove the async wrapper that with_session adds since we're already async
            inner_func = getattr(
                getattr(session_wrapped, "__wrapped__", session_wrapped),
                "__wrapped__",
                session_wrapped
            )

            # Get database session manually
            db = await app_context.get_db()
            async with db.session_factory() as session:
                return await inner_func(session, *args, **kwargs)
        else:
            # Remote mode - use API clients
            clients = {
                "index_client": IndexClient(
                    base_url=app_context.remote.server_url or "",
                    api_key=app_context.remote.api_key,
                    timeout=app_context.remote.timeout,
                    max_retries=app_context.remote.max_retries,
                    verify_ssl=app_context.remote.verify_ssl,
                ),
                "search_client": SearchClient(
                    base_url=app_context.remote.server_url or "",
                    api_key=app_context.remote.api_key,
                    timeout=app_context.remote.timeout,
                    max_retries=app_context.remote.max_retries,
                    verify_ssl=app_context.remote.verify_ssl,
                ),
            }

            try:
                # Pass clients to the command function
                return await f(*args, clients=clients, **kwargs)
            finally:
                # Clean up client connections
                for client in clients.values():
                    await client.close()

    return wrapper
