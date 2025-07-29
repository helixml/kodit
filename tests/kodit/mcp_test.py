"""Tests for STDIO MCP server functionality."""

from fastmcp import FastMCP

from kodit.mcp import create_mcp_server, register_mcp_tools


def test_mcp_components() -> None:
    """Test that the MCP components are properly configured."""
    # Create a server instance
    mcp = create_mcp_server(name="Test Server")

    # Verify it's a FastMCP instance
    assert isinstance(mcp, FastMCP)
    assert mcp.name == "Test Server"

    # Register tools
    register_mcp_tools(mcp)

    # Verify tools are registered (this is internal to FastMCP,
    # but we can check the server is still valid)
    assert isinstance(mcp, FastMCP)
