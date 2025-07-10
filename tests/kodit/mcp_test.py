"""Tests for the MCP server implementation."""

import pytest
from fastmcp import Client
from mcp.types import TextContent
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.value_objects import FileProcessingStatus
from kodit.infrastructure.sqlalchemy.entities import (
    File,
    Index,
    Snippet,
    Source,
    SourceType,
)
from kodit.mcp import mcp


@pytest.mark.asyncio
async def test_mcp_server_basic_functionality(session: AsyncSession) -> None:
    """Test basic MCP server functionality with real database."""
    # Create test data
    source = Source(
        uri="file:///test/repo",
        cloned_path="/tmp/test/repo",  # noqa: S108
        source_type=SourceType.FOLDER,
    )
    session.add(source)
    await session.flush()

    index = Index(source_id=source.id)
    session.add(index)
    await session.flush()

    file = File(
        created_at=source.created_at,
        updated_at=source.updated_at,
        source_id=source.id,
        mime_type="text/plain",
        uri="file:///test/repo/example.py",
        cloned_path="/tmp/test/repo/example.py",  # noqa: S108
        sha256="abc123",
        size_bytes=100,
        extension="py",
        file_processing_status=FileProcessingStatus.CLEAN,
    )
    session.add(file)
    await session.flush()

    snippet = Snippet(
        file_id=file.id,
        index_id=index.id,
        content="def hello_world():\n    return 'Hello, World!'",
        summary="Simple hello world function",
    )
    session.add(snippet)
    await session.commit()

    # Test MCP client connection
    async with Client(mcp) as client:
        # Test tool listing
        tools = await client.list_tools()
        assert len(tools) == 2
        tool_names = {tool.name for tool in tools}
        assert "search" in tool_names
        assert "get_version" in tool_names

        # Test version tool
        result = await client.call_tool("get_version")
        assert len(result.content) == 1
        content = result.content[0]
        assert isinstance(content, TextContent)
        assert content.text is not None

        # Test search tool
        result = await client.call_tool(
            "search",
            {
                "user_intent": "Find hello world functions",
                "related_file_paths": [],
                "related_file_contents": [],
                "keywords": ["hello", "world"],
            },
        )
        assert len(result.content) == 1
        content = result.content[0]
        assert isinstance(content, TextContent)
        assert content.text is not None
