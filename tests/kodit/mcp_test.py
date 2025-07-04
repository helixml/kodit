"""Tests for the MCP server implementation."""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from fastmcp import Client
from mcp.types import TextContent
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.config import AppContext
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
async def test_mcp_server_basic_functionality(
    session: AsyncSession, app_context: AppContext
) -> None:
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

    # Create a mock session context manager
    mock_session_context = AsyncMock()
    mock_session_context.__aenter__ = AsyncMock(return_value=session)
    mock_session_context.__aexit__ = AsyncMock(return_value=None)

    # Mock the database session factory
    mock_db = MagicMock()
    mock_db.session_factory.return_value = mock_session_context

    # Mock app context methods with proper search configuration
    mock_app_context = MagicMock()
    mock_app_context.get_clone_dir.return_value = app_context.get_clone_dir()
    mock_app_context.get_db = AsyncMock(return_value=mock_db)
    mock_app_context.default_search.provider = "sqlite"
    mock_app_context.embedding_endpoint = None
    mock_app_context.default_endpoint = None

    with patch("kodit.mcp.AppContext", return_value=mock_app_context):
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
            assert len(result) == 1
            content = result[0]
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
            assert len(result) == 1
            content = result[0]
            assert isinstance(content, TextContent)
            assert content.text is not None
