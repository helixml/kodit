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


@pytest.mark.asyncio
async def test_mcp_code_retrieval() -> None:
    """Test end-to-end flow of adding code and retrieving it via MCP."""
    # Can't quite test this properly yet, because the MCP server runs in a separate
    # process using the user's actual database. So this code is just testing that it
    # doesn't crash or anything.
    server_process = None
    try:
        # Start the server in a separate process
        server_process = Process(target=run_server, daemon=True)
        server_process.start()

        # Wait for server to start
        await wait_for_server()

        # Connect to MCP server
        async with (
            sse_client("http://127.0.0.1:8000/sse/") as (
                read_stream,
                write_stream,
            ),
            ClientSession(read_stream, write_stream) as mcp_session,
        ):
            # Initialize the connection
            await mcp_session.initialize()

            # Call retrieve_relevant_snippets tool
            await mcp_session.call_tool(
                "retrieve_relevant_snippets",
                {
                    "user_intent": "I want to find code that prints hello world",
                    "related_file_paths": [],
                    "related_file_contents": [],
                    "keywords": ["hello", "world", "print"],
                },
            )

    finally:
        if server_process:
            server_process.kill()
            server_process.join(timeout=1)
