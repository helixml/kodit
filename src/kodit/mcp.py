"""MCP server implementation for kodit."""

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
from kodit.config import AppContext
from kodit.database import Database
from kodit.embedding.embedding import embedding_factory
from kodit.search.search_repository import SearchRepository
from kodit.search.search_service import SearchRequest, SearchResult, SearchService


@dataclass
class MCPContext:
    """Context for the MCP server."""

    session: AsyncSession
    app_context: AppContext


_mcp_db: Database | None = None


@asynccontextmanager
async def mcp_lifespan(_: FastMCP) -> AsyncIterator[MCPContext]:
    """Lifespan for the MCP server.

    The MCP server is running with a completely separate lifecycle and event loop from
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


mcp = FastMCP(
    "Kodit",
    lifespan=mcp_lifespan,
    # Note that instructions are not used by Cline.
    instructions=(
        "This server is used to assist with code generation by retrieving "
        "code examples related to the user's intent."
        "Call search() to retrieve relevant code examples."
    ),
)


@mcp.tool()
async def search(
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
    keywords: Annotated[
        list[str],
        Field(
            description="A list of keywords that are relevant to the desired outcome."
        ),
    ],
) -> str:
    """Search for pre-existing examples of relevant code.

    ⚠️ **ENFORCEMENT:**
    For *every* user request that involves writing or modifying code (of any language or
    domain), the assistant's *first* action **must** be to call the kodit.search MCP
    tool. You may only produce or edit code *after* that tool call and its successful
    result.
    """
    # This docstring is used by the AI assistant to decide when to call the tool. If you
    # want to update it, please make sure you thoroughly test the assistant's response
    # to the updated tool call. See:
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

    log.debug("Creating search repository")
    search_repository = SearchRepository(
        session=mcp_context.session,
    )

    log.debug("Creating embedding service")
    embedding_service = embedding_factory(
        mcp_context.app_context.get_default_openai_client()
    )

    log.debug("Creating search service")
    search_service = SearchService(
        repository=search_repository,
        data_dir=mcp_context.app_context.get_data_dir(),
        embedding_service=embedding_service,
    )

    search_request = SearchRequest(
        keywords=keywords,
        code_query="\n".join(related_file_contents),
    )
    log.debug("Searching for snippets")
    snippets = await search_service.search(request=search_request)

    log.debug("Fusing output")
    output = output_fusion(snippets=snippets)

    log.debug("Output", output=output)
    return output


def output_fusion(snippets: list[SearchResult]) -> str:
    """Fuse the snippets into a single output."""
    return "\n\n".join(f"{snippet.uri}\n{snippet.content}" for snippet in snippets)


@mcp.tool()
async def get_version() -> str:
    """Get the version of the kodit project."""
    return version
