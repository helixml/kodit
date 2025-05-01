"""Tests for the MCP server implementation."""

from threading import Thread

import pytest
import uvicorn
from fastapi.testclient import TestClient
from mcp import ClientSession
from mcp.client.sse import sse_client

from coda.app import app


@pytest.fixture
def client() -> TestClient:
    """Create a test client for the FastAPI application."""
    return TestClient(app)


@pytest.fixture(scope="session")
def server() -> None:
    """Start the server in  separate thread for testing."""

    def run_server() -> None:
        uvicorn.run(app, host="127.0.0.1", port=8000, log_level="error")

    server_thread = Thread(target=run_server, daemon=True)
    server_thread.start()


@pytest.mark.asyncio(loop_scope="session")
async def test_mcp_client_connection(server) -> None:  # noqa: ANN001, ARG001
    """Test connecting to the MCP server using ClientSession."""
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
