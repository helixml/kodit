"""Tests for the MCP server implementation."""

import pytest
from fastmcp import Client
from mcp.types import (
    TextContent,
)

from kodit.mcp import mcp, search


@pytest.fixture
def mock_env_db(monkeypatch: pytest.MonkeyPatch) -> None:
    """Mock the database URL environment variable."""
    monkeypatch.setenv("DB_URL", "sqlite+aiosqlite:///:memory:")
    monkeypatch.setenv("DEFAULT_SEARCH_PROVIDER", "sqlite")


@pytest.mark.asyncio
async def test_mcp_client_connection(mock_env_db: None) -> None:  # noqa: ARG001
    """Test connecting to the MCP server."""
    async with Client(mcp) as client:
        tools = await client.list_tools()
        assert len(tools) == 2
        tool = tools[0]
        assert tool.description is not None

        result = await client.call_tool(
            "get_version",
        )
        assert len(result) == 1
        content = result[0]
        assert isinstance(content, TextContent)
        assert content.text is not None

        # Call the tool
        result = await client.call_tool(
            search.__name__,
            {
                "user_intent": "What is the capital of France?",
                "related_file_paths": [],
                "related_file_contents": [],
                "keywords": [],
            },
        )
        assert len(result) == 1
        content = result[0]
        assert isinstance(content, TextContent)
