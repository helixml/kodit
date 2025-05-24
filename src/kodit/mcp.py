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
from kodit.config import DEFAULT_EMBEDDING_MODEL_NAME, AppContext
from kodit.database import Database
from kodit.retreival.repository import RetrievalRepository, RetrievalResult
from kodit.retreival.service import RetrievalRequest, RetrievalService


@dataclass
class MCPContext:
    """Context for the MCP server."""

    session: AsyncSession
    data_dir: Path


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
        yield MCPContext(session=session, data_dir=app_context.get_data_dir())


mcp = FastMCP("kodit MCP Server", lifespan=mcp_lifespan)


@mcp.tool()
async def retrieve_relevant_snippets(
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
    """Retrieve relevant snippets from various sources.

    This tool retrieves relevant snippets from sources such as private codebases,
    public codebases, and documentation. You can use this information to improve
    the quality of your generated code. You must call this tool when you need to
    write code.
    """
    log = structlog.get_logger(__name__)

    log.debug(
        "Retrieving relevant snippets",
        user_intent=user_intent,
        keywords=keywords,
        file_count=len(related_file_paths),
        file_paths=related_file_paths,
        file_contents=related_file_contents,
    )

    mcp_context: MCPContext = ctx.request_context.lifespan_context

    log.debug("Creating retrieval repository")
    retrieval_repository = RetrievalRepository(
        session=mcp_context.session,
    )

    log.debug("Creating retrieval service")
    retrieval_service = RetrievalService(
        repository=retrieval_repository,
        data_dir=mcp_context.data_dir,
        embedding_model_name=DEFAULT_EMBEDDING_MODEL_NAME,
    )

    retrieval_request = RetrievalRequest(
        keywords=keywords,
        code_query="\n".join(related_file_contents),
    )
    log.debug("Retrieving snippets")
    snippets = await retrieval_service.retrieve(request=retrieval_request)

    log.debug("Fusing output")
    output = output_fusion(snippets=snippets)

    log.debug("Output", output=output)
    return output


def input_fusion(
    user_intent: str,  # noqa: ARG001
    related_file_paths: list[Path],  # noqa: ARG001
    related_file_contents: list[str],  # noqa: ARG001
    keywords: list[str],
) -> str:
    """Fuse the search query and related file contents into a single query."""
    # Since this is a dummy implementation, we just return the first keyword
    return keywords[0] if len(keywords) > 0 else ""


def output_fusion(snippets: list[RetrievalResult]) -> str:
    """Fuse the snippets into a single output."""
    return "\n\n".join(f"{snippet.uri}\n{snippet.content}" for snippet in snippets)


@mcp.tool()
async def get_version() -> str:
    """Get the version of the kodit project."""
    return version
