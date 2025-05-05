"""MCP server implementation for kodit."""

from pathlib import Path
from typing import Annotated

import structlog
from mcp.server.fastmcp import FastMCP
from pydantic import Field

mcp = FastMCP("kodit MCP Server")


@mcp.tool()
async def retrieve_relevant_snippets(
    search_query: Annotated[
        str,
        Field(description="Describe the user's intent in a few sentences."),
    ],
    related_file_paths: Annotated[
        list[Path],
        Field(
            description="A list of absolute paths to files that are relevant to the "
            "user's intent."
        ),
    ],
    related_file_contents: Annotated[
        list[str],
        Field(
            description="A list of the contents of the files that are relevant to the "
            "user's intent."
        ),
    ],
) -> str:
    """Retrieve relevant snippets from various sources.

    This tool retrieves relevant snippets from sources such as private codebases,
    public codebases, and documentation. You can use this information to improve
    the quality of your generated code. You must call this tool when you need to
    write code.
    """
    # Log the search query and related files for debugging
    log = structlog.get_logger(__name__)
    log.debug(
        "Retrieving relevant snippets",
        search_query=search_query,
        file_count=len(related_file_paths),
        file_paths=related_file_paths,
        file_contents=related_file_contents,
    )

    return "Retrieved"
