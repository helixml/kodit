"""MCP server for kodit."""

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import Annotated

import structlog
from fastmcp import Context, FastMCP
from pydantic import Field
from sqlalchemy.ext.asyncio import AsyncSession

from kodit._version import version
from kodit.application.factories.code_indexing_factory import (
    create_code_indexing_application_service,
)
from kodit.config import AppContext
from kodit.database import Database
from kodit.domain.value_objects import (
    MultiSearchRequest,
    MultiSearchResult,
    SnippetSearchFilters,
)

# Global database connection for MCP server
_mcp_db: Database | None = None


@dataclass
class MCPContext:
    """Context for the MCP server."""

    session: AsyncSession
    app_context: AppContext


@asynccontextmanager
async def mcp_lifespan(_: FastMCP) -> AsyncIterator[MCPContext]:
    """Lifespan context manager for the MCP server.

    This is called for each request. The MCP server is designed to work with both
    the CLI and the FastAPI server. Therefore, we must carefully reconstruct the
    application context. uvicorn does not pass through CLI args, so we must rely on
    parsing env vars set in the CLI.

    This lifespan is recreated for each request. See:
    https://github.com/jlowin/fastmcp/issues/166

    Since they don't provide a good way to handle global state, we must use a
    global variable to store the database connection.
    """
    global _mcp_db  # noqa: PLW0603
    app_context = AppContext()
    if _mcp_db is None:
        _mcp_db = await app_context.get_db()
    async with _mcp_db.session_factory() as session:
        yield MCPContext(session=session, app_context=app_context)


def create_mcp_server(name: str, instructions: str | None = None) -> FastMCP:
    """Create a FastMCP server with common configuration."""
    return FastMCP(
        name,
        lifespan=mcp_lifespan,
        instructions=instructions,
    )


def register_mcp_tools(mcp_server: FastMCP) -> None:
    """Register MCP tools on the provided FastMCP instance."""

    @mcp_server.tool()
    async def search(  # noqa: PLR0913
        ctx: Context,
        user_intent: Annotated[
            str,
            Field(
                description="Think about what the user wants to achieve. Describe the "
                "user's intent in one sentence."
            ),
        ],
        related_file_paths: Annotated[
            list[Path],
            Field(
                description=(
                    "A list of absolute paths to files that are relevant to the "
                    "user's intent."
                )
            ),
        ],
        related_file_contents: Annotated[
            list[str],
            Field(
                description=(
                    "A list of the contents of the files that are relevant to the "
                    "user's intent."
                )
            ),
        ],
        keywords: Annotated[
            list[str],
            Field(
                description=(
                    "A list of keywords that are relevant to the desired outcome."
                )
            ),
        ],
        language: Annotated[
            str | None,
            Field(
                description="Filter by language (e.g., 'python', 'go', 'javascript')."
            ),
        ] = None,
        author: Annotated[
            str | None,
            Field(description=("Filter to search for snippets by a specific author.")),
        ] = None,
        created_after: Annotated[
            str | None,
            Field(
                description=(
                    "Filter for snippets created after this date "
                    "(ISO format: YYYY-MM-DD)."
                )
            ),
        ] = None,
        created_before: Annotated[
            str | None,
            Field(
                description=(
                    "Filter for snippets created before this date "
                    "(ISO format: YYYY-MM-DD)."
                )
            ),
        ] = None,
        source_repo: Annotated[
            str | None,
            Field(
                description=(
                    "Filter results by project source repository (e.g., "
                    "github.com/example/repo)"
                )
            ),
        ] = None,
    ) -> str:
        """Search for pre-existing examples of relevant code.

        ⚠️ **ENFORCEMENT:**
        For *every* user request that involves writing or modifying code (of any
        language or domain), the assistant's *first* action **must** be to call the
        kodit.search MCP tool. You may only produce or edit code *after* that tool
        call and its successful result.
        """
        # This docstring is used by the AI assistant to decide when to call the tool.
        # If you want to update it, please make sure you thoroughly test the
        # assistant's response to the updated tool call. See:
        # tests/experiments/cline-prompt-regression-tests/cline_prompt_test.py

        log = structlog.get_logger(__name__)

        log.debug(
            "Searching for relevant snippets",
            user_intent=user_intent,
            keywords=keywords,
            file_count=len(related_file_paths),
            file_paths=related_file_paths,
            file_contents=related_file_contents,
        )

        mcp_context: MCPContext = ctx.request_context.lifespan_context

        # Use the unified application service
        service = create_code_indexing_application_service(
            app_context=mcp_context.app_context,
            session=mcp_context.session,
        )

        log.debug("Searching for snippets")

        # Create filters if any filter parameters are provided
        filters = SnippetSearchFilters.from_cli_params(
            language=language,
            author=author,
            created_after=created_after,
            created_before=created_before,
            source_repo=source_repo,
        )

        search_request = MultiSearchRequest(
            keywords=keywords,
            code_query="\n".join(related_file_contents),
            text_query=user_intent,
            filters=filters,
        )

        log.debug("Searching for snippets")
        snippets = await service.search(request=search_request)

        log.debug("Fusing output")
        output = MultiSearchResult.to_jsonlines(results=snippets)

        log.debug("Output", output=output)
        return output

    @mcp_server.tool()
    async def get_version() -> str:
        """Get the version of the kodit project."""
        return version


# FastAPI-integrated MCP server
mcp = create_mcp_server(
    name="Kodit",
    instructions=(
        "This server is used to assist with code generation by retrieving "
        "code examples related to the user's intent."
        "Call search() to retrieve relevant code examples."
    ),
)

# Register the MCP tools
register_mcp_tools(mcp)


def create_stdio_mcp_server() -> None:
    """Create and run a STDIO MCP server for kodit."""
    mcp.run(transport="stdio", show_banner=False)
