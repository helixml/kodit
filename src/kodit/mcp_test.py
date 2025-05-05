"""Tests for the MCP server implementation."""

from multiprocessing import Process

import httpx
import pytest
import uvicorn
from httpx_retries import Retry, RetryTransport
from mcp import ClientSession
from mcp.client.sse import sse_client

from kodit.app import app


def run_server() -> None:
    """Run the uvicorn server in a separate process."""
    config = uvicorn.Config(app, host="127.0.0.1", port=8000, log_level="error")
    server = uvicorn.Server(config)
    server.run()


async def wait_for_server() -> None:
    """Wait for the server to start."""
    retry = Retry(total=5, backoff_factor=0.5)
    transport = RetryTransport(retry=retry)

    async with httpx.AsyncClient(transport=transport) as client:
        response = await client.get("http://127.0.0.1:8000/")
        assert response.status_code == 200


@pytest.mark.asyncio
async def test_mcp_client_connection() -> None:
    """Test connecting to the MCP server using ClientSession."""
    server_process = None
    try:
        # Start the server in a separate process
        server_process = Process(target=run_server, daemon=True)
        server_process.start()

        # Ping the server with requests, wait for it to start
        await wait_for_server()

        # The sse_client returns read and write streams, not a client object
        async with (
            sse_client("http://127.0.0.1:8000/sse/") as (
                read_stream,
                write_stream,
            ),
            ClientSession(read_stream, write_stream) as session,
        ):
            # Initialize the connection
            await session.initialize()

            # List available tools
            tools_result = await session.list_tools()
            # Check that we got a proper tools result with a tools attribute
            assert hasattr(tools_result, "tools")
            # Verify we can see the 'retrieve_relevant_snippets' tool
            tool_names = [tool.name for tool in tools_result.tools]
            assert "retrieve_relevant_snippets" in tool_names
    finally:
        # Due to https://github.com/modelcontextprotocol/python-sdk/issues/514, mcp
        # doesn't shut down the server when requested. So we have to force kill it.
        server_process.kill()
        server_process.join(timeout=1)
