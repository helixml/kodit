"""Tests for the MCP server implementation."""

import pytest
from fastmcp import Client
from mcp.types import (
    TextContent,
)

from kodit.mcp import mcp


@pytest.mark.asyncio
async def test_mcp_client_connection() -> None:
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
            "retrieve_relevant_snippets",
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
