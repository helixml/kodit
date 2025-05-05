"""Server-Sent Events (SSE) implementation for kodit."""

from collections.abc import Coroutine
from typing import Any

from fastapi import Request
from mcp.server.fastmcp import FastMCP
from mcp.server.session import ServerSession
from mcp.server.sse import SseServerTransport
from starlette.applications import Starlette
from starlette.routing import Mount, Route

####################################################################################
# Temporary monkeypatch which avoids crashing when a POST message is received
# before a connection has been initialized, e.g: after a deployment.
old__received_request = ServerSession._received_request  # noqa: SLF001


async def _received_request(self: ServerSession, *args: Any, **kwargs: Any) -> None:
    """Handle a received request, catching RuntimeError to avoid crashes.

    This is a temporary monkeypatch to avoid crashing when a POST message is
    received before a connection has been initialized, e.g: after a deployment.
    """
    try:
        return await old__received_request(self, *args, **kwargs)
    except RuntimeError:
        pass


# pylint: disable-next=protected-access
ServerSession._received_request = _received_request  # noqa: SLF001
####################################################################################


def create_sse_server(mcp: FastMCP) -> Starlette:
    """Create a Starlette app that handles SSE connections and message handling."""
    transport = SseServerTransport("/messages/")

    # Define handler functions
    async def handle_sse(request: Request) -> Coroutine[Any, Any, None]:
        """Handle SSE connections."""
        async with transport.connect_sse(
            request.scope,
            request.receive,
            request._send,  # noqa: SLF001
        ) as streams:
            await mcp._mcp_server.run(  # noqa: SLF001
                streams[0],
                streams[1],
                mcp._mcp_server.create_initialization_options(),  # noqa: SLF001
            )

    # Create Starlette routes for SSE and message handling
    routes = [
        Route("/sse/", endpoint=handle_sse),
        Mount("/messages/", app=transport.handle_post_message),
    ]

    # Create a Starlette app
    return Starlette(routes=routes)
